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

func getWrongOptionsError(subCommandsMap map[string]func()) (err error) {
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

// Parse parses a list of strings as cli options and returns the final configuration.
// Returns an error if the list of strings cannot be parsed.
func Parse(args []string) (configuration.AppOptions, error) {
	rememberSubCmd := "remember"
	forgetSubCmd := "forget"
	cacheSubCmd := "cache"

	var appOptions configuration.AppOptions

	subCommandsMap := map[string]func(){
		rememberSubCmd: func() {
			rememberCmd := flag.NewFlagSet(rememberSubCmd, flag.ExitOnError)

			rememberCmd.Usage = func() {
				fmt.Printf("Usage of %s:\n", rememberSubCmd)
				fmt.Println("  On cache miss: runs the given build command as is and stores the result in the local cache")
				fmt.Println("  On cache hit: makes the passed tag point to the cache entry in the remote registry - no build is performed")
				fmt.Println("  Example:")
				fmt.Println("    mimosa remember -- docker buildx build --build-arg MYARG=MYVALUE --platform linux/amd64,linux/arm64 --push -t hytromo/mimosa-example:v1 .")
				fmt.Println()
				rememberCmd.PrintDefaults()
			}

			dryRunOpt := rememberCmd.Bool("dry-run", false, "Do not actually build or push anything - just show if it would be a cache hit or not - combine with the LOG_LEVEL env variable for more details.")
			// Parse the arguments after the subcommand
			err := rememberCmd.Parse(args[2:])
			if err != nil {
				log.Errorf("Failed to parse arguments after subcommand: %s", err)
				return
			}

			appOptions.Remember.CommandToRun = rememberCmd.Args()
			appOptions.Remember.DryRun = *dryRunOpt
			appOptions.Remember.Enabled = true
		},
		forgetSubCmd: func() {
			forgetCmd := flag.NewFlagSet(forgetSubCmd, flag.ExitOnError)

			forgetCmd.Usage = func() {
				fmt.Printf("Usage of %s:\n", forgetSubCmd)
				fmt.Println("  Forgets a specific cache entry - same arguments as the remember subcommand")
				fmt.Println("  Example:")
				fmt.Println("    mimosa forget -- docker buildx build --build-arg MYARG=MYVALUE --platform linux/amd64,linux/arm64 --push -t hytromo/mimosa-example:v1 .")
				fmt.Println()
				forgetCmd.PrintDefaults()
			}

			dryRunOpt := forgetCmd.Bool("dry-run", false, "Do not actually remove any cache entry - just show what would happen")

			err := forgetCmd.Parse(args[2:])
			if err != nil {
				log.Errorf("Failed to parse arguments after subcommand: %s", err)
				return
			}

			appOptions.Forget.CommandToRun = forgetCmd.Args()
			appOptions.Forget.DryRun = *dryRunOpt
			appOptions.Forget.Enabled = true
		},
		cacheSubCmd: func() {
			cacheCmd := flag.NewFlagSet(cacheSubCmd, flag.ExitOnError)

			cacheCmd.Usage = func() {
				fmt.Printf("Usage of %s:\n", cacheSubCmd)
				fmt.Println("  Manages the local disk cache")
				fmt.Println()
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
				return
			}

			appOptions.Cache.Forget = *forgetOpt
			appOptions.Cache.ForgetYes = *forgetYesOpt
			appOptions.Cache.Show = *showOpt
			appOptions.Cache.ToEnvValue = *toEnvValue
			appOptions.Cache.Purge = *purge
			appOptions.Cache.Enabled = true
		},
	}

	chosenCommand := "non-existent-subcommand"

	if len(args) >= 2 {
		chosenCommand = args[1]
	}

	parseCliOptionsOfSubcommand, subcommandExists := subCommandsMap[chosenCommand]

	if !subcommandExists {
		return appOptions, getWrongOptionsError(subCommandsMap)
	}

	parseCliOptionsOfSubcommand()

	return appOptions, nil
}
