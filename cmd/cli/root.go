/*
Copyright © 2022 Johnson Shi <Johnson.Shi@microsoft.com>

*/
package main

import (
	"flag"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func newRootCmd(stdin io.Reader, stdout io.Writer, stderr io.Writer, args []string) *cobra.Command {
	cobraCmd := &cobra.Command{
		Use:   "lpm",
		Short: "lpm - analyze, generate, and push layer provenance metadata (lpm) for container images",
	}

	cobraCmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)
	flags := cobraCmd.PersistentFlags()

	cobraCmd.AddCommand(
		newAnalyzeCmd(stdin, stdout, stderr, args),
		newConfigAnnotateCmd(stdin, stdout, stderr, args),
	)

	_ = flags.Parse(args)

	return cobraCmd
}

func execute() {
	rootCmd := newRootCmd(os.Stdin, os.Stdout, os.Stderr, os.Args[1:])
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
