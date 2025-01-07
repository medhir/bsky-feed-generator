package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/medhir/bsky-feed-generator/service/pkg/auth"
	"github.com/medhir/bsky-feed-generator/service/pkg/db"
	"github.com/medhir/bsky-feed-generator/service/pkg/feedrouter"
	staticfeed "github.com/medhir/bsky-feed-generator/service/pkg/feeds/static"
	ginendpoints "github.com/medhir/bsky-feed-generator/service/pkg/gin"
	"github.com/medhir/bsky-feed-generator/service/pkg/stream"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"time"

	ginprometheus "github.com/ericvolp12/go-gin-prometheus"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Configure feed generator from environment variables

	// Registers a tracer Provider globally if the exporter endpoint is set
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" {
		log.Println("initializing tracer...")
		shutdown, err := installExportPipeline(ctx)
		if err != nil {
			log.Fatal(err)
		}
		defer func() {
			if err := shutdown(ctx); err != nil {
				log.Fatal(err)
			}
		}()
	}

	feedActorDID := os.Getenv("FEED_ACTOR_DID")
	if feedActorDID == "" {
		log.Fatal("FEED_ACTOR_DID environment variable must be set")
	}

	// serviceEndpoint is a URL that the feed generator will be available at
	serviceEndpoint := os.Getenv("SERVICE_ENDPOINT")
	if serviceEndpoint == "" {
		log.Fatal("SERVICE_ENDPOINT environment variable must be set")
	}

	// Set the acceptable DIDs for the feed generator to respond to
	// We'll default to the feedActorDID and the Service Endpoint as a did:web
	serviceURL, err := url.Parse(serviceEndpoint)
	if err != nil {
		log.Fatal(fmt.Errorf("error parsing service endpoint: %w", err))
	}

	serviceWebDID := "did:web:" + serviceURL.Hostname()

	log.Printf("service DID Web: %s", serviceWebDID)

	acceptableDIDs := []string{feedActorDID, serviceWebDID}

	// Create a new feed router instance
	feedRouter, err := feedrouter.NewFeedRouter(ctx, feedActorDID, serviceWebDID, acceptableDIDs, serviceEndpoint)
	if err != nil {
		log.Fatal(fmt.Errorf("error creating feed router: %w", err))
	}

	// Here we can add feeds to the Feed Router instance
	// Feeds conform to the Feed interface, which is defined in
	// pkg/feedrouter/feedrouter.go

	// For demonstration purposes, we'll use a static feed generator
	// that will always return the same feed skeleton (one post)
	staticFeed, staticFeedAliases, err := staticfeed.NewStaticFeed(
		ctx,
		feedActorDID,
		"static",
		// This static post is the conversation that sparked this demo repo
		[]string{"at://did:plc:q6gjnaw2blty4crticxkmujt/app.bsky.feed.post/3jx7msc4ive26"},
	)

	// Add the static feed to the feed generator
	feedRouter.AddFeed(staticFeedAliases, staticFeed)

	// Create a gin router with default middleware for logging and recovery
	router := gin.Default()

	// Plug in OTEL Middleware and skip metrics endpoint
	router.Use(
		otelgin.Middleware(
			"go-bsky-feed-generator",
			otelgin.WithFilter(func(req *http.Request) bool {
				return req.URL.Path != "/metrics"
			}),
		),
	)

	// Add Prometheus metrics middleware
	p := ginprometheus.NewPrometheus("gin", nil)
	p.Use(router)

	// Add unauthenticated routes for feed generator
	ep := ginendpoints.NewEndpoints(feedRouter)
	router.GET("/.well-known/did.json", ep.GetWellKnownDID)
	router.GET("/xrpc/app.bsky.feed.describeFeedGenerator", ep.DescribeFeeds)

	// Plug in Authentication Middleware
	auther, err := auth.NewAuth(
		100_000,
		time.Hour*12,
		5,
		serviceWebDID,
	)
	if err != nil {
		log.Fatalf("Failed to create Auth: %v", err)
	}

	router.Use(auther.AuthenticateGinRequestViaJWT)

	// Add authenticated routes for feed generator
	router.GET("/xrpc/app.bsky.feed.getFeedSkeleton", ep.GetFeedSkeleton)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting server on port %s", port)
	go func() {
		if err := router.Run(fmt.Sprintf(":%s", port)); err != nil {
			log.Printf("HTTP server error: %v", err)
			cancel()
		}
	}()

	dbInstance, err := db.NewDB(ctx)
	if err != nil {
		log.Fatalf("Failed to create DB: %v", err)
	}
	logger := slog.Default()
	subscriber, err := stream.NewSubscriber(ctx, dbInstance, logger)
	if err != nil {
		log.Fatalf("Failed to create subscriber: %v", err)
	}
	// Run subscriber in main goroutine
	for {
		if err := subscriber.Run(); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("Subscriber error: %v, retrying in 1 second...", err)
			time.Sleep(1 * time.Second)
			continue
		}
		break
	}
}

// installExportPipeline registers a trace provider instance as a global trace provider,
func installExportPipeline(ctx context.Context) (func(context.Context) error, error) {
	client := otlptracehttp.NewClient()
	exporter, err := otlptrace.New(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("creating OTLP trace exporter: %w", err)
	}

	tracerProvider := newTraceProvider(exporter)
	otel.SetTracerProvider(tracerProvider)

	return tracerProvider.Shutdown, nil
}

// newTraceProvider creates a new trace provider instance.
func newTraceProvider(exp sdktrace.SpanExporter) *sdktrace.TracerProvider {
	// Ensure default SDK resources and the required service name are set.
	r, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("go-bsky-feed-generator"),
		),
	)

	if err != nil {
		panic(err)
	}

	// initialize the traceIDRatioBasedSampler to sample all traces
	traceIDRatioBasedSampler := sdktrace.TraceIDRatioBased(1)

	return sdktrace.NewTracerProvider(
		sdktrace.WithSampler(traceIDRatioBasedSampler),
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(r),
	)
}
