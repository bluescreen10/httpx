package httpx

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

// ParseBody parses the HTTP request body into a struct based on the Content-Type header.
//
// The function automatically detects the content type and applies the appropriate
// parsing strategy:
//   - Form data (application/x-www-form-urlencoded, multipart/form-data, text/plain):
//     Uses struct tags with `form:"fieldname"` to map form fields to struct fields
//   - JSON (application/json): Uses json.Unmarshal with `json` struct tags
//   - XML (application/xml): Uses xml.Unmarshal with `xml` struct tags
//
// The dst parameter must be a pointer to a struct.
//
// For form data parsing, the function supports the following field types:
//   - string
//   - int, int8, int16, int32, int64
//   - uint, uint8, uint16, uint32, uint64
//   - float32, float64
//   - bool (accepts various representations like "on"/"off", "1"/"0", "yes"/"no", "true"/"false")
//   - slices of the above types
//
// Form struct tags support the following format:
//
//	`form:"fieldname,option1,option2"`
//
// Available options:
//   - required: Field must be present in the form data
//
// Returns an error if:
//   - The destination is not a pointer to a struct
//   - The content type is not supported
//   - Required fields are missing (form data only)
//   - Field values cannot be converted to the target type
//   - The request body cannot be read or parsed
func ParseBody(r *http.Request, dst any) error {
	switch r.Header.Get("Content-Type") {
	case "application/x-www-form-urlencoded", "multipart/form-data", "text/plain":
		return parseBodyForm(r, dst)
	case "application/json":
		return parseBodyJSON(r, dst)
	case "application/xml":
		return parseBodyXML(r, dst)
	default:
		return fmt.Errorf("content type not supported")
	}
}

// parseBodyForm parses form data from the HTTP request into a struct.
//
// This function handles application/x-www-form-urlencoded, multipart/form-data,
// and text/plain content types. It uses reflection to examine the destination
// struct and maps form fields based on `form` struct tags.
//
// The function validates required fields and converts string values to the
// appropriate Go types using bindFieldValue.
func parseBodyForm(r *http.Request, dst any) error {
	if err := r.ParseForm(); err != nil {
		return fmt.Errorf("failed to parse form: %w", err)
	}

	rv := reflect.ValueOf(dst)
	if rv.Kind() != reflect.Ptr {
		return errors.New("destination must be a pointer to a struct")
	}

	rv = rv.Elem()
	if rv.Kind() != reflect.Struct {
		return errors.New("destination must be a pointer to a struct")
	}

	rt := rv.Type()

	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		fieldType := rt.Field(i)

		if !field.CanSet() {
			continue
		}

		formTag := fieldType.Tag.Get("form")
		if formTag == "" || formTag == "-" {
			continue
		}

		tagParts := strings.Split(formTag, ",")
		fieldName := tagParts[0]

		required := false
		for _, option := range tagParts[1:] {
			if option == "required" {
				required = true
				break
			}
		}

		formValues := r.Form[fieldName]

		if required && len(formValues) == 0 {
			return fmt.Errorf("required field '%s' is missing", fieldName)
		}

		if len(formValues) == 0 {
			continue
		}

		if err := bindFieldValue(field, formValues); err != nil {
			return fmt.Errorf("failed to bind field '%s': %w", fieldName, err)
		}
	}

	return nil
}

// bindFieldValue converts and assigns form values to a struct field.
func bindFieldValue(field reflect.Value, values []string) error {
	if len(values) == 0 {
		return nil
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(values[0])

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		val, err := strconv.ParseInt(values[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid integer value: %s", values[0])
		}
		field.SetInt(val)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		val, err := strconv.ParseUint(values[0], 10, 64)
		if err != nil {
			return fmt.Errorf("invalid unsigned integer value: %s", values[0])
		}
		field.SetUint(val)

	case reflect.Float32, reflect.Float64:
		val, err := strconv.ParseFloat(values[0], 64)
		if err != nil {
			return fmt.Errorf("invalid float value: %s", values[0])
		}
		field.SetFloat(val)

	case reflect.Bool:
		val, err := strconv.ParseBool(values[0])
		if err != nil {
			// Handle common HTML form boolean representations
			switch strings.ToLower(values[0]) {
			case "on", "1", "yes", "true":
				val = true
			case "off", "0", "no", "false", "":
				val = false
			default:
				return fmt.Errorf("invalid boolean value: %s", values[0])
			}
		}
		field.SetBool(val)

	case reflect.Slice:
		sliceType := field.Type()

		slice := reflect.MakeSlice(sliceType, len(values), len(values))

		for i, value := range values {
			elem := slice.Index(i)
			if err := bindFieldValue(elem, []string{value}); err != nil {
				return fmt.Errorf("failed to bind slice element at index %d: %w", i, err)
			}
		}

		field.Set(slice)
		return nil

	default:
		return fmt.Errorf("unsupported field type: %s", field.Kind())
	}

	return nil
}

// parseBodyJSON parses JSON data from the HTTP request body into a struct.
//
// This function reads the entire request body and uses json.Unmarshal
// to decode the JSON data into the destination struct. The struct should
// use `json` struct tags to control field mapping.
//
// Returns an error if the request body cannot be read or if JSON
// unmarshaling fails.
func parseBodyJSON(r *http.Request, dst any) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, dst)
}

// parseBodyXML parses XML data from the HTTP request body into a struct.
//
// This function reads the entire request body and uses xml.Unmarshal
// to decode the XML data into the destination struct. The struct should
// use `xml` struct tags to control field mapping.
//
// Returns an error if the request body cannot be read or if XML
// unmarshaling fails.
func parseBodyXML(r *http.Request, dst any) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return xml.Unmarshal(body, dst)
}
