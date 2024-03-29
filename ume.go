package main

import (
	"context"
	"crypto/sha1"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/ssocreds"
	"github.com/aws/aws-sdk-go-v2/service/sso"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
	"github.com/olekukonko/tablewriter"
	"gopkg.in/ini.v1"
)

func listProfileNames() []string {
	config, err := ini.Load(config.DefaultSharedConfigFilename())
	if err != nil {
		log.Fatal(err)
	}
	var profiles []string
	for _, v := range config.Sections() {
		if !strings.HasPrefix(v.Name(), "profile ") {
			continue
		}
		profiles = append(profiles, strings.Replace(v.Name(), "profile ", "", 1))
	}
	return profiles
}

func getConfig(profile string) aws.Config {
	config, err := config.LoadDefaultConfig(context.TODO(), config.WithSharedConfigProfile(profile))
	if err != nil {
		log.Fatal(err)
	}
	return config
}

func getSharedConfig(profile string) config.SharedConfig {
	sharedConfig, err := config.LoadSharedConfigProfile(context.TODO(), profile)
	if err != nil {
		log.Fatal(err)
	}
	return sharedConfig
}

func getSharedConfigs() []config.SharedConfig {
	var configs []config.SharedConfig
	for _, profile := range listProfileNames() {
		configs = append(configs, getSharedConfig(profile))
	}
	return configs
}

func convertStsToGenericCredentials(credentials types.Credentials) aws.Credentials {
	return aws.Credentials{
		AccessKeyID:     *credentials.AccessKeyId,
		SecretAccessKey: *credentials.SecretAccessKey,
		SessionToken:    *credentials.SessionToken,
		Expires:         *credentials.Expiration,
	}
}

func prettyPrintSharedConfigs() {
	fmt.Println("Listing...")
	table := tablewriter.NewWriter(os.Stdout)
	table.SetBorder(false)
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetHeader([]string{"PROFILE", "TYPE", "SOURCE", "MFA?", "REGION", "ACCOUNT"})
	for _, c := range getSharedConfigs() {
		var profile_type string
		var source_profile string
		profile_account_id := "Unavailable"
		if c.RoleARN != "" {
			profile_type = "Role"
			profile_account_id = strings.Split(c.RoleARN, ":")[4]
			if c.SourceProfileName != "" {
				source_profile = c.SourceProfileName
			} else {
				source_profile = "None"
			}
		} else if c.SSOSessionName != "" {
			profile_type = "SSO"
			profile_account_id = c.SSOAccountID
			source_profile = c.SSOSessionName
		} else {
			profile_type = "User"
			source_profile = "None"
		}

		var mfa_needed string
		if c.MFASerial != "" {
			mfa_needed = "Yes"
		} else {
			mfa_needed = "No"
		}

		table.Append([]string{c.Profile, profile_type, source_profile, mfa_needed, c.Region, profile_account_id})
	}

	table.Render()
}

func fetchSSOCredentials(profile config.SharedConfig) aws.Credentials {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithSharedConfigProfile(profile.Profile),
	)
	if err != nil {
		log.Fatal(err)
	}

	client := sso.NewFromConfig(cfg)

	var provider aws.CredentialsProvider
	provider = ssocreds.New(client, profile.SSOAccountID, profile.SSORoleName, profile.SSOSession.SSOStartURL, func(options *ssocreds.Options) {
		// The standard appears to be to name the cache file using the sha1 of the start url.
		// However it falls back to using the session name, which is how the AWS CLI behaves.
		// TODO Support both, or manage all credentials ourselves
		data := []byte(profile.SSOSessionName)
		path := path.Join(config.DefaultSharedConfigFilename(), "../sso/cache", fmt.Sprintf("%x.json", sha1.Sum(data)))
		options.CachedTokenFilepath = path
	})
	provider = aws.NewCredentialsCache(provider)
	creds, err := provider.Retrieve(context.TODO())
	if err != nil {
		log.Fatal(err)
	}
	return creds
}

func exportToWrapper(credentials aws.Credentials, profile string, region string) {
	if region == "" || profile == "" {
		log.Fatal("Region or profile not set")
	}
	sessionToken := credentials.SessionToken
	if sessionToken == "" {
		sessionToken = "None"
	}
	var expires string
	if credentials.CanExpire {
		expires = credentials.Expires.Format("2006-01-02T15:04:05")
	} else {
		expires = "None"
	}
	fmt.Printf("%s %s %s %s %s %s %s %s\n", "Awsume", credentials.AccessKeyID, credentials.SecretAccessKey, sessionToken, region, "None", profile, expires)
}

func main() {
	listProfiles := flag.Bool("l", false, "List profiles")
	unset := flag.Bool("u", false, "Unset AWS environment variables")
	flag.Parse()

	if *listProfiles {
		prettyPrintSharedConfigs()
	} else if *unset {
		fmt.Println("Unset")
		os.Exit(0)
	} else if flag.NArg() == 1 {
		profileName := flag.Arg(0)

		c := getSharedConfig(profileName)

		var credentials aws.Credentials
		targetProfileName := profileName
		targetProfile := c

		if targetProfile.SourceProfileName != "" {
			targetProfileName = targetProfile.SourceProfileName
			targetProfile = getSharedConfig(targetProfileName)

			config := getConfig(targetProfileName)
			client := sts.NewFromConfig(config)

			sessionName := "ume-session"
			roleSession, err := client.AssumeRole(context.TODO(), &sts.AssumeRoleInput{
				RoleArn:         &c.RoleARN,
				RoleSessionName: &sessionName,
			})

			if err != nil {
				log.Fatal(err)
			}

			credentials = convertStsToGenericCredentials(*roleSession.Credentials)
		} else if targetProfile.SSOSessionName != "" {
			credentials = fetchSSOCredentials(targetProfile)
		} else {
			credentials = targetProfile.Credentials
		}

		greenColour := "\033[32m"
		resetColour := "\033[0m"
		exportToWrapper(credentials, profileName, targetProfile.Region)
		fmt.Fprintf(os.Stderr, "%s[%s] Role credentials will expire %s%s\n", greenColour, profileName, credentials.Expires, resetColour)
	}

}
