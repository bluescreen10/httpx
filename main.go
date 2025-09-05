package httpx

// func main() {
// 	mux := http.NewServeMux()

// 	mux.HandleFunc("GET /", handleRenderForm)
// 	mux.HandleFunc("POST /", handleSubmitForm)

// 	http.ListenAndServe(":8080", mux)
// }

// func handleRenderForm(w http.ResponseWriter, r *http.Request) {
// 	form := `
// 	<html>
// 	<body>
// 	<h1>Awesome Form</h1>
// 	<form method="POST">
// 	   <input type="text" name="email" />
// 	   <input type="submit" />
// 	</form>
// 	</body>
// 	</html>`
// 	w.Header().Set("Content-Type", "text/html")
// 	w.Header().Set("Content-Length", strconv.Itoa(len(form)))
// 	w.WriteHeader(200)
// 	w.Write([]byte(form))
// }

// func handleSubmitForm(w http.ResponseWriter, r *http.Request) {
// 	type user struct {
// 		Email string `form:"email,required"`
// 	}
// 	u := user{}
// 	if err := form.Bind(r, &u); err != nil {
// 		w.WriteHeader(500)
// 		fmt.Fprintf(w, "error parsing form: %w", err)
// 	} else {
// 		w.WriteHeader(200)
// 		fmt.Fprintf(w, "email: %s", u.Email)
// 	}
// }
