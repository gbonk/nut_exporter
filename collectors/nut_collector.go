package collectors

import (
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	nut "github.com/robbiet480/go.nut"
)

var deviceLabels = []string{"model", "mfr", "serial", "type", "description", "contact", "location", "part", "macaddr"}

type NutCollector struct {
	deviceDesc *prometheus.Desc
	logger     *slog.Logger
	opts       *NutCollectorOpts
	onRegex    *regexp.Regexp
	offRegex   *regexp.Regexp
}

type NutCollectorOpts struct {
	Namespace         string
	Server            string
	ServerPort        int
	Ups               string
	Username          string
	Password          string
	Variables         []string
	Statuses          []string
	OnRegex           string
	OffRegex          string
	DisableDeviceInfo bool
}

func NewNutCollector(opts NutCollectorOpts, logger *slog.Logger) (*NutCollector, error) {

	logger.Debug("NewNutCollector: Start", "NutCollectionOps: ", opts)

	deviceDesc := prometheus.NewDesc(prometheus.BuildFQName(opts.Namespace, "", "device_info"),
		"UPS Device information",
		deviceLabels, nil,
	)
	if opts.DisableDeviceInfo {
		deviceDesc = nil
	}

	var onRegex, offRegex *regexp.Regexp
	var err error

	if opts.OnRegex != "" {
		onRegex, err = regexp.Compile(fmt.Sprintf("(?i)%s", opts.OnRegex))
		if err != nil {
			return nil, err
		}
	}

	if opts.OffRegex != "" {
		offRegex, err = regexp.Compile(fmt.Sprintf("(?i)%s", opts.OffRegex))
		if err != nil {
			return nil, err
		}
	}

	collector := &NutCollector{
		deviceDesc: deviceDesc,
		logger:     logger,
		opts:       &opts,
		onRegex:    onRegex,
		offRegex:   offRegex,
	}

	if opts.Ups != "" {
		valid, err := collector.IsValidUPSName(opts.Ups)
		if err != nil {
			logger.Warn("NewNutCollector: Error detected while verifying UPS name - proceeding without validation", "error", err)
		} else if !valid {
			return nil, fmt.Errorf("NewNutCollector:  %s UPS is not a valid name in the NUT server %s", opts.Ups, opts.Server)
		}
	}

	logger.Info("NewNutCollector:  collector configured", "variables", strings.Join(collector.opts.Variables, ","))
	return collector, nil
}

// The Testing Connection Hook ---

// Define a function type that mirrors the signature of nut.Connect
type connectFunc func(server string, port int) (NutClient, error)

// defaultNutConnect dials the real network socket using the original library
var defaultNutConnect connectFunc = func(server string, port int) (NutClient, error) {

	slog.Debug("connectFunc: Performing Live Network connection")

	client, err := nut.Connect(server, port)

	if err != nil {

		slog.Debug("connectFunc: Error Calling nut.Connect:", "err", err)

		return nil, err
	}
	return &realNutClient{client: &client}, nil
}

// nutConnectHook is what your Collect() function calls.
// In your test files, you will overwrite this variable with a mock connection closer!
var nutConnectHook = defaultNutConnect

func (c *NutCollector) Collect(ch chan<- prometheus.Metric) {
	c.logger.Debug("Collect: Start")

	if ch == nil {

		c.logger.Error("Collect: Channel 'ch'is nil")
		panic("Collect: Channel 'ch'is nil")

	}

	c.logger.Debug("Collect: Connecting to server", "server", c.opts.Server, "port", c.opts.ServerPort)

	client, err := nutConnectHook(c.opts.Server, c.opts.ServerPort)

	if err != nil {
		c.logger.Error("Collect: Error when calling nutConnectHook:", "err", err)
		ch <- prometheus.NewInvalidMetric(
			prometheus.NewDesc(prometheus.BuildFQName(c.opts.Namespace, "", "error"),
				"Collect: Failure gathering UPS variables", nil, nil),
			err)
		return
	}

	defer client.Disconnect()
	c.logger.Debug("Collect: Connected to server", "server", c.opts.Server)

	if c.opts.Username != "" && c.opts.Password != "" {
		_, err = client.Authenticate(c.opts.Username, c.opts.Password)
		if err == nil {
			c.logger.Debug("Collect: Authenticated", "server", c.opts.Server, "user", c.opts.Username)
		} else {
			c.logger.Warn("Collect: Failed to authenticate to NUT server", "server", c.opts.Server, "user", c.opts.Username)
			//Don't bail after logging the warning. Most NUT configurations do not require authn to read variables
		}
	}

	upsList := []NutUPS{}
	if c.opts.Ups != "" {

		c.logger.Debug("Collect: Opts UPS is", "ups", c.opts.Ups)

		ups, err := client.NewUPS(c.opts.Ups)

		if err != nil {
			c.logger.Error("Collect: Failure instantiating the UPS", "name", c.opts.Ups, "err", err)
			ch <- prometheus.NewInvalidMetric(
				prometheus.NewDesc(prometheus.BuildFQName(c.opts.Namespace, "", "error"),
					"Collect: Failure instantiating the UPS", nil, nil),
				err)
			return
		}

		c.logger.Debug("Collect: Instantiated UPS", "name", c.opts.Ups)
		upsList = append(upsList, ups)
	} else {

		c.logger.Debug("Collect: Opts UPS is empty")

		upsList, err = client.GetUPSList()

		if err != nil {

			c.logger.Error("Collect: Failure getting the list of UPS devices", "err", err)
			ch <- prometheus.NewInvalidMetric(
				prometheus.NewDesc(prometheus.BuildFQName(c.opts.Namespace, "", "error"),
					"Collect: Failure getting the list of UPS devices", nil, nil),
				err)
			return
		}

		c.logger.Debug("Collect: Obtained list of UPS devices")
		//		upsList = tmp
		for _, ups := range upsList {
			c.logger.Debug("Collect: UPS name detection", "name", ups.GetName())
		}
	}

	if len(upsList) > 1 {
		c.logger.Error("Collect: Multiple UPS devices were found by NUT for this scrape. For this configuration, you MUST scrape this exporter with a query string parameter indicating which UPS to scrape. Valid values of ups are:")
		for _, ups := range upsList {
			c.logger.Error(ups.GetName())
		}
		ch <- prometheus.NewInvalidMetric(
			prometheus.NewDesc(prometheus.BuildFQName(c.opts.Namespace, "", "error"),
				"Collect:  Multiple UPS devices were found from NUT. Please add a ups=<name> query string", nil, nil),
			err)
		return
	} else if len(upsList) == 1 {
		//Set the name so subsequent scrapes don't have to look it up
		c.opts.Ups = upsList[0].GetName()
	} else if len(upsList) == 0 {
		c.logger.Debug("Collect:  The length of upsList is now Zero")
	}

	for _, ups := range upsList {
		device := make(map[string]string)
		for _, label := range deviceLabels {
			device[label] = ""
		}

		c.logger.Debug(
			"Collect: UPS info",
			"name", ups.GetName(),
			"description", ups.GetDescription(),
			"master", ups.GetMaster(),
			"nmumber_of_logins", ups.GetNumberOfLogins(),
		)
		for i, clientName := range ups.GetClients() {
			c.logger.Debug(fmt.Sprintf("Collect: client %d", i), "name", clientName)
		}
		for _, command := range ups.GetCommands() {
			c.logger.Debug("Collect: UPS Command", "command", command.Name, "description", command.Description)
		}
		for _, variable := range ups.GetVariables() {
			c.logger.Debug("Collect: Current UPS Variable: ", "variable", variable)
			path := strings.Split(variable.Name, ".")
			if path[0] == "device" {
				device[path[1]] = fmt.Sprintf("%v", variable.Value)
			}

			/* Done special processing - now get as general as possible and gather all requested or number-like metrics */

			// Check if the target matches any regex pattern in the slice
			hasMatch := slices.ContainsFunc(c.opts.Variables, func(pattern string) bool {

				c.logger.Debug("Collect: hasMatch", "pattern", pattern, "name", variable.Name)

				matched, _ := regexp.MatchString(pattern, variable.Name)
				return matched
			})

			//			if len(c.opts.Variables) == 0 || slices.Contains(c.opts.Variables, variable.Name) {
			if hasMatch {
				c.logger.Debug("Collect: Variable Name match found in variable filter.")
				c.logger.Debug("Collect: Exporting Variable: " + variable.Name)
				value := float64(0)

				/* Deal with ups.status specially because it is a collection of 'flags' */
				if variable.Name == "ups.status" {
					setStatuses := make(map[string]bool)
					varDesc := prometheus.NewDesc(prometheus.BuildFQName(c.opts.Namespace, "", strings.Replace(variable.Name, ".", "_", -1)),
						fmt.Sprintf("%s (%s)", variable.Description, variable.Name),
						[]string{"flag"}, nil,
					)

					for _, statusFlag := range strings.Split(variable.Value.(string), " ") {
						setStatuses[statusFlag] = true
						ch <- prometheus.MustNewConstMetric(varDesc, prometheus.GaugeValue, float64(1), statusFlag)
					}

					/* If the user specifies the statues that must always be set, handle that here */
					if len(c.opts.Statuses) > 0 {
						for _, status := range c.opts.Statuses {
							/* This status flag was set because we saw it in the output... skip it */
							if _, ok := setStatuses[status]; ok {
								continue
							}
							ch <- prometheus.MustNewConstMetric(varDesc, prometheus.GaugeValue, float64(0), status)
						}
					}
					continue
				}

				/* This is overkill - the library only deals with bool, string, int64 and float64 */
				switch v := variable.Value.(type) {
				case bool:
					if v {
						value = float64(1)
					}
				case int:
					value = float64(v)
				case int8:
					value = float64(v)
				case int16:
					value = float64(v)
				case int64:
					value = float64(v)
				case float32:
					value = float64(v)
				case float64:
					value = float64(v)
				case string:
					/* All numbers should be coaxed to native types by the library, so see if we can figure out
					   if this string could possible represent a binary value
					*/
					if c.onRegex != nil && c.onRegex.MatchString(variable.Value.(string)) {
						c.logger.Debug("Collect: Converted string to 1 due to regex match", "value", variable.Value.(string))
						value = float64(1)
					} else if c.offRegex != nil && c.offRegex.MatchString(variable.Value.(string)) {
						c.logger.Debug("Collect: Converted string to 0 due to regex match", "value", variable.Value.(string))
						value = float64(0)
					} else {
						c.logger.Debug("Collect: Cannot convert string to binary 0/1", "value", variable.Value.(string))
						continue
					}
				default:
					c.logger.Warn("Collect: Unknown variable type from nut client library", "name", variable.Name, "type", fmt.Sprintf("%T", v), "claimed_type", variable.Type, "value", v)
					continue
				}

				name := strings.ReplaceAll(variable.Name, ".", "_")
				name = strings.ReplaceAll(name, "-", "_")

				fqName := prometheus.BuildFQName(c.opts.Namespace, "", name)
				varDesc := prometheus.NewDesc(fqName,
					fmt.Sprintf("%s (%s)", variable.Description, variable.Name),
					nil, nil,
				)

				c.logger.Debug("Collect: Collecting as prometheus metric", "name", fqName, "value", value)
				ch <- prometheus.MustNewConstMetric(varDesc, prometheus.GaugeValue, value)
			} else {
				c.logger.Debug("Collect: Skipping variable: " + variable.Name)
			}
		}

		// Only provide device info if not disabled
		if !c.opts.DisableDeviceInfo {
			deviceValues := []string{}
			for _, label := range deviceLabels {
				deviceValues = append(deviceValues, device[label])
			}
			ch <- prometheus.MustNewConstMetric(c.deviceDesc, prometheus.GaugeValue, float64(1), deviceValues...)
		}
	}
}

func (c *NutCollector) Describe(ch chan<- *prometheus.Desc) {
	if !c.opts.DisableDeviceInfo {
		ch <- c.deviceDesc
	}
}

func (c *NutCollector) IsValidUPSName(upsName string) (bool, error) {
	result := false

	c.logger.Debug(fmt.Sprintf("IsValidUPSName: Connecting to server and verifying `%s` is a valid UPS name", upsName), "server", c.opts.Server)

	//	client, err := nut.Connect(c.opts.Server)
	client, err := nutConnectHook(c.opts.Server, c.opts.ServerPort)

	if err != nil {
		c.logger.Error("IsValidUPSName: error while connecting to server", "err", err)
		return result, err
	}

	defer client.Disconnect()

	if c.opts.Username != "" && c.opts.Password != "" {
		_, err = client.Authenticate(c.opts.Username, c.opts.Password)
		if err != nil {
			c.logger.Warn("IsValidUPSName: Failed to authenticate to NUT server", "server", c.opts.Server, "user", c.opts.Username)
			//Don't bail after logging the warning. Most NUT configurations do not require authn to get the UPS list
		}
	}

	upsList, err := client.GetUPSList()
	if err != nil {
		c.logger.Error("IsValidUPSName: Failure getting the list of UPS devices", "err", err)
		return result, err
	}

	for _, ups := range upsList {
		c.logger.Debug("IsValidUPSName: UPS name detection", "name", ups.GetName())
		if ups.GetName() == upsName {
			result = true
		}
	}

	c.logger.Debug(fmt.Sprintf("IsValidUPSName: Validity result for UPS named `%s`", upsName), "valid", result)
	return result, nil
}
