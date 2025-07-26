package argsparser

import (
	"errors"
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

	return errors.New(
		fmt.Sprintln(
			"Please specify one of the valid subcommands:",
			strings.Join(allSubcommands, ", "),
			"\nYou can use the -h/--help switch on the subcommands for further assistance on their usage",
		),
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
	cacheSubCmd := "cache"

	var appOptions configuration.AppOptions

	subCommandsMap := map[string]func(){
		rememberSubCmd: func() {
			rememberCmd := flag.NewFlagSet(rememberSubCmd, flag.ExitOnError)
			// Parse the arguments after the subcommand
			err := rememberCmd.Parse(args[2:])
			if err != nil {
				log.Errorf("Failed to parse arguments after subcommand: %s", err)
				return
			}

			appOptions.Remember.CommandToRun = rememberCmd.Args()
			appOptions.Remember.Enabled = true
		},
		cacheSubCmd: func() {
			cacheCmd := flag.NewFlagSet(cacheSubCmd, flag.ExitOnError)
			forgetOpt := cacheCmd.String("forget", "", "forget all cache entries older than a period of time (e.g. 1h, 2d, 3w)")
			forgetYesOpt := cacheCmd.Bool("yes", false, "skip confirmation prompt for forgetting cache")
			showOpt := cacheCmd.Bool("show", false, "show the cache location")
			toEnvValue := cacheCmd.Bool("to-env-value", false, "combine the existing disk cache with the MIMOSA_CACHE env variable")
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
