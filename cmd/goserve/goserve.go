package main

import (
	"fmt"
	"goserve/pkg/file"
	"goserve/pkg/handler"
	"goserve/pkg/middleware"
	"net/http"
	"path/filepath"

	"github.com/spf13/cobra"
)

var defaultConfig = GoserveConfig{
	Port:            1234,
	LogEnabled:      false,
	DownloadEnabled: false,
}

type GoserveConfig struct {
	Port            int
	LogEnabled      bool
	DownloadEnabled bool
}

func DefaultConfig() GoserveConfig {
	return defaultConfig
}

func RootCmd(config GoserveConfig) *cobra.Command {
	if config.Port == 0 {
		config.Port = defaultConfig.Port
	}
	rootCmd := &cobra.Command{
		Use:     "goserve <filepath>",
		Short:   "Simple static file server",
		Long:    "Simple static file server.",
		Version: "1.0.0",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var fpath string
			if len(args) == 0 {
				fpath = "."
			} else {
				fpath = args[0]
			}
			fpath, err := filepath.Abs(fpath)
			if err != nil {
				return err
			}
			root, err := file.GetFileTree(cmd.ErrOrStderr(), fpath)
			if err != nil {
				return err
			}
			errch := make(chan error)
			httphandler := handler.ServeFileTree(root, config.DownloadEnabled, cmd.Version, errch)
			if config.LogEnabled {
				httphandler = middleware.Logger(cmd.OutOrStdout(), httphandler)
			}
			var serveMode string
			if config.DownloadEnabled {
				serveMode = "raw"
			} else {
				serveMode = "download"
			}
			addr := fmt.Sprintf(":%d", config.Port)
			cmd.Printf("serving %s [%s] at http://localhost%s\n", serveMode, root.Path, addr)
			go func() {
				for err := range errch {
					cmd.PrintErrln(err)
				}
			}()
			return http.ListenAndServe(addr, httphandler)
		},
	}
	rootCmd.Flags().IntVarP(&config.Port, "port", "p", config.Port, "port to listen on")
	rootCmd.Flags().BoolVarP(&config.LogEnabled, "log", "l", config.LogEnabled, "whether to log requests to stdout or not")
	rootCmd.Flags().BoolVarP(&config.DownloadEnabled, "download", "d", config.DownloadEnabled, "whether to serve content to download or raw")
	return rootCmd
}
