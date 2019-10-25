package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/youyo/tfstore"
)

var Version string

var rootCmd = &cobra.Command{
	Use:          "tfstore",
	Short:        "Create standard backend (S3+Dyanamodb) for terraform state files.",
	Version:      Version,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		stackName := viper.GetString("stack-name")

		tf := tfstore.New()

		if err := tf.Create(ctx, stackName); err != nil {
			return err
		}

		fmt.Println("Creating stack...")

		if err := tf.WaitCreation(ctx, stackName); err != nil {
			return err
		}

		if err := tf.GetOutput(ctx, stackName); err != nil {
			return err
		}

		exampleCommand := tf.GenerateCommandExample(ctx)

		fmt.Println("")
		fmt.Println("bucket: " + tf.BucketName)
		fmt.Println("dynamodb_table: " + tf.DynamodbTableName)
		fmt.Println("region: " + tf.Region)
		fmt.Println("key: terraform.tfstate")
		fmt.Println("encrypt: true")
		fmt.Println("")
		fmt.Println("Terraform initialize command is")
		fmt.Println("")
		fmt.Println("$ " + exampleCommand)

		return nil
	},
}

// Execute
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.Flags().StringP("stack-name", "n", "tfstore", "The name that is associated with the stack.")
	viper.BindPFlags(rootCmd.Flags())
}

func initConfig() {}
