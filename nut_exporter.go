package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"nut/collectors"
	"os"
	"strconv"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	promcollectors "github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/exporter-toolkit/web"
	"github.com/prometheus/exporter-toolkit/web/kingpinflag"
)

func init() {
	// // Override Go's global resolver
	// net.DefaultResolver = &net.Resolver{
	// 	PreferGo: true, // Forces Go's native DNS resolver over CGO
	// 	Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
	// 		// address will contain the DNS server being hit (e.g., "127.0.0.53:53")
	// 		fmt.Printf("🔍 [Global Resolver] Intercepted network lookup query at %s\n", time.Now().Format(time.RFC3339Nano))
	// 		slog.Debug("🔍 Context", "ctx", ctx, "network", network, "address", address)

	// 		// Dial out using a standard dialer to fulfill the original query
	// 		d := net.Dialer{}
	// 		return d.DialContext(ctx, network, address)
	// 	},
	// }

	// net.Dialer = &net.Dialer{
	// 	Control: func(network, address string, c syscall.RawConn) error {
	// 		fmt.Printf("🎯 Dialing address: %s over %s\n", address, network)
	// 		return nil
	// 	},
	// }
}

var Version = "testing"

var (
	server = kingpin.Flag(
		"nut.server", "Hostname or IP address of the server to connect to. ($NUT_EXPORTER_SERVER)",
	).Envar("NUT_EXPORTER_SERVER").Default("127.0.0.1").String()

	serverport = kingpin.Flag(
		"nut.serverport", "Port on the NUT server to connect to. ($NUT_EXPORTER_SERVERPORT)",
	).Envar("NUT_EXPORTER_SERVERPORT").Default("3493").Int()

	nutUsername = kingpin.Flag(
		"nut.username", "If set, will authenticate with this username to the server. Password must be set in NUT_EXPORTER_PASSWORD environment variable. ($NUT_EXPORTER_USERNAME)",
	).Envar("NUT_EXPORTER_USERNAME").String()
	nutPassword = ""

	disableDeviceInfo = kingpin.Flag(
		"nut.disable_device_info", "A flag to disable the generation of the device_info meta metric. ($NUT_EXPORTER_DISABLE_DEVICE_INFO)",
	).Envar("NUT_EXPORTER_DISABLE_DEVICE_INFO").Default("false").Bool()

	enableFilter = kingpin.Flag(
		"nut.vars_enable", "A comma-separated list of variable names to monitor. See the variable notes in README. ($NUT_EXPORTER_VARIABLES)",
	).Envar("NUT_EXPORTER_VARIABLES").Default("battery.charge,battery.voltage,battery.voltage.nominal,input.voltage,input.voltage.nominal,ups.load,ups.status").String()

	onRegex = kingpin.Flag(
		"nut.on_regex", "This regular expression will be used to determine if the var's value should be coaxed to 1 if it is a string. Match is case-insensitive. ($NUT_EXPORTER_ON_REGEX)",
	).Envar("NUT_EXPORTER_ON_REGEX").Default("^(enable|enabled|on|true|active|activated)$").String()

	offRegex = kingpin.Flag(
		"nut.off_regex", "This regular expression will be used to determine if the var's value should be coaxed to 0 if it is a string. Match is case-insensitive. ($NUT_EXPORTER_OFF_REGEX)",
	).Envar("NUT_EXPORTER_OFF_REGEX").Default("^(disable|disabled|off|false|inactive|deactivated)$").String()

	statusList = kingpin.Flag(
		"nut.statuses", "A comma-separated list of statuses labels that will always be set by the exporter. If NUT does not set these flags, the exporter will force the network_ups_tools_ups_status{flag=\"NAME\"} to 0. See the ups.status notes in README.' ($NUT_EXPORTER_STATUSES)",
	).Envar("NUT_EXPORTER_STATUSES").Default("OL,OB,LB,HB,RB,CHRG,DISCHRG,BYPASS,CAL,OFF,OVER,TRIM,BOOST,FSD,SD").String()

	metricsNamespace = kingpin.Flag(
		"metrics.namespace", "Metrics Namespace ($NUT_EXPORTER_METRICS_NAMESPACE)",
	).Envar("NUT_EXPORTER_METRICS_NAMESPACE").Default("network_ups_tools").String()

	tookitFlags = kingpinflag.AddFlags(kingpin.CommandLine, ":9199")

	upsNutMetricsPath = kingpin.Flag(
		"web.telemetry-path", "Path under which to expose the UPS Prometheus metrics ($NUT_EXPORTER_WEB_TELEMETRY_PATH)",
	).Envar("NUT_EXPORTER_WEB_TELEMETRY_PATH").Default("/ups_metrics").String()

	exporterMetricsPath = kingpin.Flag(
		"web.exporter-telemetry-path", "Path under which to expose process metrics about this exporter ($NUT_EXPORTER_WEB_EXPORTER_TELEMETRY_PATH)",
	).Envar("NUT_EXPORTER_WEB_EXPORTER_TELEMETRY_PATH").Default("/metrics").String()

	printMetrics = kingpin.Flag(
		"printMetrics", "Print the metrics this exporter exposes and exits. Default: false ($NUT_EXPORTER_PRINT_METRICS)",
	).Envar("NUT_EXPORTER_PRINT_METRICS").Default("false").Bool()

	logLevel = kingpin.Flag(
		"log.level", "Minimum log level for messages. One of error, warn, info, or debug. Default: info ($NUT_EXPORTER_LOG_LEVEL)",
	).Envar("NUT_EXPORTER_LOG_LEVEL").Default("info").String()

	logJson = kingpin.Flag(
		"log.json", "Format log lines as JSON. Default: false ($NUT_EXPORTER_LOG_LEVEL_JSON)",
	).Envar("NUT_EXPORTER__LOG_JSON").Bool()
)

var collectorOpts collectors.NutCollectorOpts

var logger *slog.Logger = slog.New(slog.NewTextHandler(os.Stdout, nil))

func initLogger(loggerA *slog.Logger) {
	logger = loggerA
}

func init() {
	prometheus.MustRegister(promcollectors.NewBuildInfoCollector())
}

type upsNutMetricsHandler struct {
	handlers map[string]*http.Handler
}

func (h *upsNutMetricsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	logger.Debug("ServeHTTP: Received Request: " + r.URL.Path)
	if len(r.URL.Query()) > 0 {
		logger.Debug("ServeHTTP: Received Query: ", "query", r.URL.Query())
	}

	thisCollectorOpts := collectorOpts
	thisCollectorOpts.Ups = r.URL.Query().Get("ups")

	if r.URL.Query().Get("server") != "" {
		thisCollectorOpts.Server = r.URL.Query().Get("server")
	}

	if r.URL.Query().Get("serverport") != "" {
		port, err := strconv.Atoi(r.URL.Query().Get("serverport"))
		if err == nil {
			thisCollectorOpts.ServerPort = port
		} else {
			logger.Error("ServeHTTP: Unable to convert query serverport to int", "err", err)
		}
	}

	if r.URL.Query().Get("username") != "" {
		thisCollectorOpts.Username = r.URL.Query().Get("username")
	}

	if r.URL.Query().Get("password") != "" {
		thisCollectorOpts.Password = r.URL.Query().Get("password")
	}

	if r.URL.Query().Get("variables") != "" {
		thisCollectorOpts.Variables = strings.Split(r.URL.Query().Get("variables"), ",")
	}

	if r.URL.Query().Get("statuses") != "" {
		thisCollectorOpts.Statuses = strings.Split(r.URL.Query().Get("statuses"), ",")
	}

	logger.Debug("ServeHTTP: Local Collector Opts:", "ops", thisCollectorOpts)

	var promHandler http.Handler
	cacheName := fmt.Sprintf("%s:%d/%s", thisCollectorOpts.Server, thisCollectorOpts.ServerPort, thisCollectorOpts.Ups)
	if tmp, ok := h.handlers[cacheName]; ok {
		logger.Debug(fmt.Sprintf("ServeHTTP: Using existing handler for UPS `%s`", cacheName))
		promHandler = *tmp
	} else {
		//Build a custom registry to include only the UPS metrics on the UPS metrics path
		logger.Info(fmt.Sprintf("ServeHTTP: Creating new registry, handler, and collector for UPS `%s`", cacheName))
		registry := prometheus.NewRegistry()
		promHandler = promhttp.HandlerFor(registry, promhttp.HandlerOpts{Registry: registry})
		promHandler = promhttp.InstrumentMetricHandler(registry, promHandler)

		nutCollector, err := collectors.NewNutCollector(thisCollectorOpts, logger)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("500 - InternalServer Error"))
			logger.Error("ServeHTTP:  Internal server error", "err", err)
			return
		}
		registry.MustRegister(nutCollector)
		h.handlers[cacheName] = &promHandler
	}

	promHandler.ServeHTTP(w, r)
}

func main() {
	//flag.AddFlags(kingpin.CommandLine, promlogConfig)
	kingpin.Version(Version)
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	/* Reconfigure logger after parsing arguments */
	opts := &slog.HandlerOptions{}
	switch *logLevel {
	case "error":
		opts.Level = slog.LevelError
	case "warn":
		opts.Level = slog.LevelWarn
	case "info":
		opts.Level = slog.LevelInfo
	case "debug":
		opts.Level = slog.LevelDebug
	}
	if *logJson {
		logger = slog.New(slog.NewJSONHandler(os.Stderr, opts))
	} else {
		logger = slog.New(slog.NewTextHandler(os.Stderr, opts))
	}

	slog.SetDefault(logger)

	if *nutUsername != "" {
		logger.Debug("Authenticating to NUT server")
		nutPassword = os.Getenv("NUT_EXPORTER_PASSWORD")
		if nutPassword == "" {
			logger.Error("Username set, but NUT_EXPORTER_PASSWORD environment variable missing. Cannot authenticate!")
			os.Exit(2)
		}
	}

	variables := []string{}
	hasUpsStatusVariable := false
	for _, varName := range strings.Split(*enableFilter, ",") {
		// Be nice and clear spaces for those that like them
		variable := strings.Trim(varName, " ")
		if variable == "" {
			continue
		}
		variables = append(variables, variable)

		// Special handling because this is an important and commonly needed variable
		if variable == "ups.status" {
			hasUpsStatusVariable = true
		}
	}

	if !hasUpsStatusVariable {
		logger.Warn("Exporter has been started without `ups.status` variable to be exported with --nut.vars_enable. Online/offline/etc statuses will not be reported!")
	}

	statuses := []string{}
	for _, status := range strings.Split(*statusList, ",") {
		// Be nice and clear spaces for those that like them
		stat := strings.Trim(status, " ")
		if stat == "" {
			continue
		}
		statuses = append(statuses, strings.Trim(stat, " "))
	}

	collectorOpts = collectors.NutCollectorOpts{
		Namespace:         *metricsNamespace,
		Server:            *server,
		ServerPort:        *serverport,
		Username:          *nutUsername,
		Password:          nutPassword,
		DisableDeviceInfo: *disableDeviceInfo,
		Variables:         variables,
		Statuses:          statuses,
		OnRegex:           *onRegex,
		OffRegex:          *offRegex,
	}

	fmt.Printf("CollectorOps from CMD Line: %#v\n", collectorOpts)

	if *printMetrics {
		/* Make a channel and function to send output along */
		var out chan *prometheus.Desc
		eatOutput := func(in <-chan *prometheus.Desc) {
			for desc := range in {
				/* Weaksauce... no direct access to the variables */
				//Desc{fqName: "the_name", help: "help text", constLabels: {}, variableLabels: []}
				tmp := desc.String()
				vals := strings.Split(tmp, `"`)
				fmt.Printf("  %s - %s\n", vals[1], vals[3])
			}
		}

		/* Interesting juggle here...
		   - Make a channel the describe function can send output to
		   - Start the printing function that consumes the output in the background
		   - Call the describe function to feed the channel (which blocks until the consume function eats a message)
		   - When the describe function exits after returning the last item, close the channel to end the background consume function
		*/
		fmt.Println("NUT")
		nutCollector, _ := collectors.NewNutCollector(collectorOpts, logger)
		out = make(chan *prometheus.Desc)
		go eatOutput(out)
		nutCollector.Describe(out)
		close(out)

		os.Exit(0)
	}

	logger.Info("Starting nut_exporter", "version", Version)

	upsNutHandler := &upsNutMetricsHandler{
		handlers: make(map[string]*http.Handler),
	}

	http.Handle(*upsNutMetricsPath, upsNutHandler)
	http.Handle(*exporterMetricsPath, promhttp.Handler())

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>NUT Exporter</title></head>
             <body>
             <h1>NUT Exporter</h1>
             <p><a href='` + *upsNutMetricsPath + `'>UPS metrics</a></p>
             <p><a href='` + *exporterMetricsPath + `'>Exporter metrics</a></p>
             </body>
             </html>`))
	})

	srv := &http.Server{}
	if err := web.ListenAndServe(srv, tookitFlags, logger); err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}
