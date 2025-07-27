package argsparser

import (
	"flag"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/hytromo/mimosa/internal/configuration"
	log "github.com/sirupsen/logrus"
)

func getInvalidSubcommandError(subCommandsMap map[string](func() error)) (err error) {
	allSubcommands := make([]string, len(subCommandsMap))

	i := 0
	for subcommand := range subCommandsMap {
		allSubcommands[i] = subcommand
		i++
	}

	return fmt.Errorf(
		"please specify one of the valid subcommands: %s\nyou can use the -h/--help switch for further assistance on their usage",
		strings.Join(allSubcommands, ", "),
	)
}

func ParseDuration(s string) (time.Duration, error) {
	neg := false
	if len(s) > 0 && s[0] == '-' {
		neg = true
		s = s[1:]
	}

	re := regexp.MustCompile(`(\d*\.\d+|\d+)[^\d]*`)
	unitMap := map[string]time.Duration{
		"d": 24,
		"D": 24,
		"w": 7 * 24,
		"W": 7 * 24,
		"M": 30 * 24,
		"y": 365 * 24,
		"Y": 365 * 24,
	}

	strs := re.FindAllString(s, -1)
	var sumDur time.Duration
	for _, str := range strs {
		var _hours time.Duration = 1
		for unit, hours := range unitMap {
			if strings.Contains(str, unit) {
				str = strings.ReplaceAll(str, unit, "h")
				_hours = hours
				break
			}
		}

		dur, err := time.ParseDuration(str)
		if err != nil {
			return 0, err
		}

		sumDur += dur * _hours
	}

	if neg {
		sumDur = -sumDur
	}
	return sumDur, nil
}

var rememberUsage = `Usage of remember:
  On cache miss: runs the given build command as is and stores the result in the local cache
  On cache hit: makes the passed tag point to the cache entry in the remote registry - no build is performed
  Example:
	mimosa remember -- docker buildx build --build-arg MYARG=MYVALUE --platform linux/amd64,linux/arm64 --push -t hytromo/mimosa-example:v1 .
`

var forgetUsage = `Usage of forget:
  Forgets a specific cache entry
  Example:
	mimosa forget -- docker buildx build --build-arg MYARG=MYVALUE --platform linux/amd64,linux/arm64 --push -t hytromo/mimosa-example:v1 .
`

var cacheUsage = `Usage of cache:
  Manages the local disk cache
  Example:
    mimosa cache --show
	mimosa cache --forget 6M
	mimosa cache --purge
`

// Parse parses a list of strings as cli options and returns the final configuration.
// Returns an error if the list of strings cannot be parsed.
func Parse(args []string) (configuration.AppOptions, error) {
	rememberSubCmd := "remember"
	forgetSubCmd := "forget"
	cacheSubCmd := "cache"

	var appOptions configuration.AppOptions

	subCommandsMap := map[string](func() error){
		rememberSubCmd: func() error {
			rememberCmd := flag.NewFlagSet(rememberSubCmd, flag.ContinueOnError)

			rememberCmd.Usage = func() {
				fmt.Println(rememberUsage)
				rememberCmd.PrintDefaults()
			}

			dryRunOpt := rememberCmd.Bool("dry-run", false, "Do not actually build or push anything - just show if it would be a cache hit or not - combine with the LOG_LEVEL env variable for more details.")
			// Parse the arguments after the subcommand
			err := rememberCmd.Parse(args[2:])
			if err != nil {
				log.Errorf("Failed to parse arguments after subcommand: %s", err)
				return err
			}

			appOptions.Remember.CommandToRun = rememberCmd.Args()
			appOptions.Remember.DryRun = *dryRunOpt
			appOptions.Remember.Enabled = true
			return nil
		},
		forgetSubCmd: func() error {
			forgetCmd := flag.NewFlagSet(forgetSubCmd, flag.ContinueOnError)

			forgetCmd.Usage = func() {
				fmt.Println(forgetUsage)
				forgetCmd.PrintDefaults()
			}

			dryRunOpt := forgetCmd.Bool("dry-run", false, "Do not actually remove any cache entry - just show what would happen")

			err := forgetCmd.Parse(args[2:])
			if err != nil {
				log.Errorf("Failed to parse arguments after subcommand: %s", err)
				return err
			}

			appOptions.Forget.CommandToRun = forgetCmd.Args()
			appOptions.Forget.DryRun = *dryRunOpt
			appOptions.Forget.Enabled = true

			return nil
		},
		cacheSubCmd: func() error {
			cacheCmd := flag.NewFlagSet(cacheSubCmd, flag.ContinueOnError)

			cacheCmd.Usage = func() {
				fmt.Println(cacheUsage)
				cacheCmd.PrintDefaults()
			}

			forgetOpt := cacheCmd.String("forget", "", "forget all cache entries older than a period of time (e.g. 1h, 2d, 3w)")
			forgetYesOpt := cacheCmd.Bool("yes", false, "skip confirmation prompt for forgetting cache (including purging)")
			showOpt := cacheCmd.Bool("show", false, "show the cache location")
			toEnvValue := cacheCmd.Bool("to-env-value", false, "combine the existing disk cache with the MIMOSA_CACHE env variable")
			purge := cacheCmd.Bool("purge", false, "delete all cache entries")
			// Parse the arguments after the subcommand
			err := cacheCmd.Parse(args[2:])
			if err != nil {
				log.Errorf("Failed to parse arguments after subcommand: %s", err)
				return err
			}

			appOptions.Cache.Forget = *forgetOpt
			appOptions.Cache.ForgetYes = *forgetYesOpt
			appOptions.Cache.Show = *showOpt
			appOptions.Cache.ToEnvValue = *toEnvValue
			appOptions.Cache.Purge = *purge
			appOptions.Cache.Enabled = true

			return nil
		},
	}

	chosenCommand := "non-existent-subcommand"

	if len(args) >= 2 {
		chosenCommand = args[1]
	}

	parseCliOptionsOfSubcommand, subcommandExists := subCommandsMap[chosenCommand]

	if !subcommandExists {
		return appOptions, getInvalidSubcommandError(subCommandsMap)
	}

	err := parseCliOptionsOfSubcommand()

	return appOptions, err
}
