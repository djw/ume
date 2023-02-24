package main

import (
	"context"
	"flag"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/olekukonko/tablewriter"
	"gopkg.in/ini.v1"
)

func listConfigNames() []string {
	config, err := ini.Load(config.DefaultSharedConfigFilename())
	if err != nil {
		log.Fatal(err)
	}
	var profiles []string
	for _, v := range config.Sections() {
		if v.Name() == "DEFAULT" {
			continue
		}
		profiles = append(profiles, strings.Replace(v.Name(), "profile ", "", 1))
	}
	return profiles
}

func getConfigs() []config.SharedConfig {
	var configs []config.SharedConfig
	for _, profile := range listConfigNames() {
		sharedConfig, err := config.LoadSharedConfigProfile(context.TODO(), profile)
		if err != nil {
			log.Fatal(err)
		}
		configs = append(configs, sharedConfig)
	}
	return configs
}

func prettyPrintConfigs() {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetBorder(false)
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetHeader([]string{"PROFILE", "TYPE", "SOURCE", "MFA?", "REGION", "ACCOUNT"})
	for _, c := range getConfigs() {
		var profile_type string
		var source_profile string
		if c.RoleARN != "" {
			profile_type = "Role"
			if c.SourceProfileName != "" {
				source_profile = c.SourceProfileName
			} else {
				source_profile = "None"
			}
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

		profile_account_id := "Unavailable" // TODO
		table.Append([]string{c.Profile, profile_type, source_profile, mfa_needed, c.Region, profile_account_id})
	}

	table.Render()
}

func main() {
	listProfiles := flag.Bool("l", false, "List profiles")
	flag.Parse()

	if *listProfiles {
		prettyPrintConfigs()
	}
}
