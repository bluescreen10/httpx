<script>
    var ts;
    var es = new EventSource('/_livereload');

    es.onmessage = function (event) {
        var data = event.data;

        if (data.startsWith('ts=')) {
            if (ts === undefined) {
                ts = data;
            } else if (ts !== data) {
                console.log('Reloading page...');
                location.reload();
            }
        };
    }

    window.addEventListener('beforeunload', function () {
        es.close();
    });
</script>