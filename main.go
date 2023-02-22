package main

import (
	"bytes"
	"context"
	"fmt"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
)

const name = "fib"

func Fibonacci(n uint) (uint64, error) {
	if n <= 1 {
		return uint64(n), nil
	}

	if n > 93 {
		return 0, fmt.Errorf("unsupported fibonacci number %d: too large", n)
	}

	var n2, n1 uint64 = 0, 1
	for i := uint(2); i < n; i++ {
		n2, n1 = n1, n1+n2
	}

	return n2 + n1, nil
}

func handlePost(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	_, span := otel.Tracer(name).Start(ctx, "POST")
	defer span.End()
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(r.Body)
	if len(buf.String()) == 0 {
		_, err := fmt.Fprintf(w, "You didn't input anything")
		if err != nil {
			_ = fmt.Errorf("how could this be a problem")
		}
		return
	}
	post, _ := strconv.Atoi(buf.String())
	_, _ = fmt.Fprintf(w, "The number you posted was %d", post)
}

func handleQuery(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	var span trace.Span
	ctx, span = otel.Tracer(name).Start(ctx, "GET")
	defer span.End()

	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(r.Body)
	if len(buf.String()) == 0 {
		_, _ = fmt.Fprintf(w, "You didn't input anything")
		return
	}
	query, _ := strconv.ParseUint(buf.String(), 10, 32)
	result, err := func(ctx context.Context) (uint64, error) {
		_, span := otel.Tracer(name).Start(ctx, "Fibonacci")
		defer span.End()
		f, err := Fibonacci(uint(query))
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		return f, err
	}(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(w, "An error occured during calculation: %v", err)
	} else {
		_, _ = fmt.Fprintf(w, "The fibonacci number of %d is %d", query, result)
	}
	//result, _ := Fibonacci(uint(query))

}

func RouterHelper(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	newCtx, span := otel.Tracer(name).Start(ctx, "Route")
	defer span.End()
	if r.Method == "POST" {
		handlePost(w, r, newCtx)
	} else {
		handleQuery(w, r, newCtx)
	}
}

func Router(w http.ResponseWriter, r *http.Request) {
	RouterHelper(w, r, context.Background())
}

func newExporter(w io.Writer) (sdktrace.SpanExporter, error) {
	return stdouttrace.New(
		stdouttrace.WithWriter(w),
		stdouttrace.WithPrettyPrint(),
		stdouttrace.WithoutTimestamps(),
	)
}

func newResource() *resource.Resource {
	r, _ := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("fib"),
			semconv.ServiceVersionKey.String("v0.1.0"),
			attribute.String("environment", "demo"),
		),
	)
	return r
}

func main() {
	//r := mux.NewRouter()
	//r.HandleFunc("/", handlePost).Methods("POST")
	//r.HandleFunc("/", handleQuery).Methods("GET")

	//http.HandleFunc("/bar", func(w http.ResponseWriter, r *http.Request) {
	//	fmt.Fprintf(w, "Hello, %q", html.EscapeString(r.URL.Path))
	//})
	l := log.New(os.Stdout, "", 0)

	// Write telemetry data to a file.
	f, err := os.Create("traces.txt")
	if err != nil {
		l.Fatal(err)
	}
	defer f.Close()

	exp, err := newExporter(f)
	if err != nil {
		l.Fatal(err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(newResource()),
	)
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			l.Fatal(err)
		}
	}()
	otel.SetTracerProvider(tp)
	http.HandleFunc("/", Router)
	log.Fatal(http.ListenAndServe(":8080", nil))

}
