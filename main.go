package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type Map map[string]interface{}

var (
	Reset = "\033[0m"
	Bold  = "\033[1m"
	Red   = "\033[31m"
	Green = "\033[32m"
	/*
		Yellow = "\033[33m"
		Blue   = "\033[34m"
		Purple = "\033[35m"
		Cyan   = "\033[36m"
		Gray   = "\033[37m"
		White  = "\033[97m"
	*/

	version string = "0.1"

	flagProfile string
	flagRegion  string
	flagNoColor bool
	flagVersion bool
	flagForce   bool

	prefixError string

	awsAccountId      string
	awsAccountUser    string
	awsAccountMfa     string // Account MFA SerialNumber
	sessionExpiration *time.Time
)

func main() {
	defer os.Clearenv()

	flag.StringVar(&flagProfile, "profile", "", "Use a specific profile from your credential file.")
	flag.StringVar(&flagRegion, "region", "", "The region to use. Overrides config/env settings.")
	flag.BoolVar(&flagForce, "force", false, "Forces the behavior to overwrite the already defined environment variables.")
	flag.BoolVar(&flagNoColor, "no-color", false, "Turn off the color output.")
	flag.BoolVar(&flagVersion, "version", false, "Print the version and exit.")
	flag.Parse()

	if runtime.GOOS == "windows" || flagNoColor {
		Reset = ""
		Bold = ""
		Red = ""
		Green = ""
		/*
			Yellow = ""
			Blue = ""
			Purple = ""
			Cyan = ""
			Gray = ""
			White = ""
		*/
	}
	prefixError = fmt.Sprintf("%s%sERROR%s: ", Red, Bold, Reset)

	if flagVersion {
		fmt.Printf("%sAWS-MFA%s, version %s\n", Bold, Reset, version)
		fmt.Println("Written by Gustavo Knuppe - https://github.com/knuppe/aws-bash")
		fmt.Println("")
		fmt.Println("This is free software: you are free to change and redistribute it.")
		fmt.Println("There is NO WARRANTY, to the extent permitted by law.")
		os.Exit(0)
	}

	if _, err := exec.LookPath("aws"); err != nil {
		fmt.Printf("%sThe AWS CLI is not installed.\n", prefixError)
		os.Exit(1)
	}

	if flagForce {
		os.Unsetenv("AWS_ACCOUNT")
		os.Unsetenv("AWS_ACCESS_KEY_ID")
		os.Unsetenv("AWS_SECRET_ACCESS_KEY")
		os.Unsetenv("AWS_SESSION_TOKEN")
	} else if os.Getenv("AWS_ACCOUNT") != "" || os.Getenv("AWS_ACCESS_KEY_ID") != "" {
		fmt.Printf("%sThe AWS environment variables are already defined, unable to use aws-bash.\n", prefixError)
		os.Exit(1)
	}

	if flagRegion != "" {
		os.Setenv("AWS_DEFAULT_REGION", flagRegion)
	}

	identity, err := aws("sts", "get-caller-identity")
	if err != nil {
		fmt.Printf("%sUnable get the caller identity from the AWS cli: %v\n", prefixError, err)
		os.Exit(1)
	}

	if account, ok := identity.String("Account"); ok {
		os.Setenv("AWS_ACCOUNT", account)
		awsAccountId = account
	} else {
		fmt.Printf("%sUnable to retrieve the Account from sts get-caller-identity.\n", prefixError)
		os.Exit(1)
	}

	if arn, ok := identity.String("Arn"); ok && strings.ContainsRune(arn, '/') {
		awsAccountUser = strings.Split(arn, "/")[1]
	} else {
		fmt.Printf("%sUnable to retrieve the Account Resource Name from sts get-caller-identity.\n", prefixError)
		os.Exit(1)
	}

	// checks if the user has mfa enabled: aws iam list-mfa-devices
	if value, err := aws("iam", "list-mfa-devices"); err == nil {
		if devices, ok := value.Array("MFADevices"); !ok {
			fmt.Printf("%sUnable to retrieve the MFA devices from sts get-caller-identity.\n", prefixError)
			os.Exit(1)
		} else if len(devices) == 0 {
			fmt.Printf("%sThe user %s does not have any MFA device assigned.\n", prefixError, awsAccountUser)
			os.Exit(1)
		} else {
			for _, device := range devices {
				if userName, ok := device.String("UserName"); !ok || userName != awsAccountUser {
					continue
				} else if serialNumber, ok := device.String("SerialNumber"); ok {
					awsAccountMfa = serialNumber
					break
				}
			}
		}
	}

	if awsAccountMfa != "" {
		fmt.Printf("Authenticating %s@%s\n", awsAccountUser, awsAccountId)
		fmt.Printf("%s%sToken from MFA%s: ", Green, Bold, Reset)

		reader := bufio.NewReader(os.Stdin)
		tokenCode, _ := reader.ReadString('\n')

		tokenCode = strings.TrimSpace(tokenCode)
		if tokenCode == "" {
			fmt.Printf("%sThe MFA is required to authenticate the user %s.\n", prefixError, awsAccountUser)
			os.Exit(1)
		}

		if data, err := aws("sts", "get-session-token", "--serial-number", awsAccountMfa, "--token-code", tokenCode); err != nil {
			fmt.Printf("%sUnable to get the session token:\n%v\n", prefixError, err)
			os.Exit(1)
		} else if creds, ok := data.Map("Credentials"); !ok {
			fmt.Printf("%sInvalid response from GetSessionToken operation.\n", prefixError)
			os.Exit(1)
		} else {

			if id, ok := creds.String("AccessKeyId"); ok {
				os.Setenv("AWS_ACCESS_KEY_ID", id)
			} else {
				fmt.Printf("%sUnable to get the AccessKeyId from the GetSessionToken operation.\n", prefixError)
				os.Exit(1)
			}

			if secret, ok := creds.String("SecretAccessKey"); ok {
				os.Setenv("AWS_SECRET_ACCESS_KEY", secret)
			} else {
				fmt.Printf("%sUnable to get the SecretAccessKey from the GetSessionToken operation.\n", prefixError)
				os.Exit(1)
			}

			if token, ok := creds.String("SessionToken"); ok {
				os.Setenv("AWS_SESSION_TOKEN", token)
			} else {
				fmt.Printf("%sUnable to get the SessionToken from the GetSessionToken operation.\n", prefixError)
				os.Exit(1)
			}

			if exp, ok := creds.Time("Expiration"); ok {
				sessionExpiration = &exp
			}
			fmt.Printf("%sSession token obtained successfully.%s\n\n", Green, Reset)
		}
	}

	fmt.Printf("Running a bash with the AWS environment variables.\n\n")

	var ctx context.Context
	var cancel context.CancelFunc

	if sessionExpiration != nil {
		dur := time.Until(*sessionExpiration)
		ctx, cancel = context.WithTimeout(context.Background(), dur)
		defer cancel()

	} else {
		ctx = context.Background()
	}

	var cmd *exec.Cmd

	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd.exe")
	} else {
		cmd = exec.CommandContext(ctx, "bash")
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()

	if err == context.DeadlineExceeded {
		fmt.Printf("%sThe authentication token expired.\n", prefixError)
		os.Exit(1)
	}

	fmt.Printf("\nExiting the bash with AWS environment variables.\n")
}

func aws(a ...string) (Map, error) {

	if flagProfile != "" {
		a = append(a, "--profile", flagProfile)
	}

	if flagRegion != "" {
		a = append(a, "--region", flagRegion)
	}

	a = append(a, "--output", "json")

	cmd := exec.Command("aws", a...)
	data, err := cmd.CombinedOutput()
	if err != nil {
		if s := strings.TrimSpace(string(data)); len(s) > 0 {
			return nil, errors.New(s)
		}
		return nil, err
	}

	out := make(Map)
	if err = json.Unmarshal(data, &out); err != nil {
		return nil, err
	}

	return out, nil
}

func (m Map) Array(key string) ([]Map, bool) {
	if v, ok := m[key]; !ok {
		return nil, false
	} else if items, ok := v.([]interface{}); !ok {
		return nil, false
	} else {
		value := make([]Map, len(items))
		for index, item := range items {
			if v, ok := item.(map[string]interface{}); ok {
				value[index] = v
			}
		}
		return value, true
	}
}
func (m Map) String(key string) (string, bool) {
	if v, ok := m[key]; !ok {
		return "", false
	} else if s, ok := v.(string); !ok {
		return "", false
	} else {
		return s, true
	}
}
func (m Map) Map(key string) (Map, bool) {
	if v, ok := m[key]; !ok {
		return nil, false
	} else if value, ok := v.(map[string]interface{}); !ok {
		return nil, false
	} else {
		return value, true
	}
}
func (m Map) Time(key string) (time.Time, bool) {
	if s, ok := m.String(key); !ok {
		return time.Time{}, false
	} else if t, err := time.Parse(time.RFC3339, s); err != nil {
		return time.Time{}, false
	} else {
		return t, true
	}
}
