package goserve

import (
	"crypto/tls"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/cmgsj/goserve/pkg/files"
	"github.com/cmgsj/goserve/pkg/middleware/logging"
)

var version = "dev"

func NewCommandGoserve() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "goserve",
		Short: "HTTP file server.",
		CompletionOptions: cobra.CompletionOptions{
			HiddenDefaultCmd: true,
		},
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          cobra.ExactArgs(1),
		RunE:          run,
		Version:       version,
	}

	cmd.Flags().String("exclude", "", "exclude file pattern")
	cmd.Flags().String("host", "", "http host")
	cmd.Flags().String("log-format", "text", "log format (json|text)")
	cmd.Flags().String("log-level", "info", "log level (debug|info|warn|error)")
	cmd.Flags().Uint64("port", 0, "http port")
	cmd.Flags().String("tls-cert", "", "tls cert file")
	cmd.Flags().String("tls-key", "", "tls key file")
	cmd.Flags().Bool("uploads", false, "enable uploads")
	cmd.Flags().String("uploads-dir", "", "uploads directory")
	cmd.Flags().Bool("uploads-timestamp", false, "add upload timestamp")

	viper.AutomaticEnv()
	viper.AllowEmptyEnv(true)
	viper.SetEnvPrefix("goserve")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.BindPFlags(cmd.Flags())

	return cmd
}

func run(cmd *cobra.Command, args []string) error {
	exclude := viper.GetString("exclude")
	host := viper.GetString("host")
	logFormat := viper.GetString("log-format")
	logLevel := viper.GetString("log-level")
	port := viper.GetUint64("port")
	tlsCert := viper.GetString("tls-cert")
	tlsKey := viper.GetString("tls-key")
	uploads := viper.GetBool("uploads")
	uploadsDir := viper.GetString("uploads-dir")
	uploadsTimestamp := viper.GetBool("uploads-timestamp")

	err := initLogger(loggerOptions{
		format: logFormat,
		level:  logLevel,
	})
	if err != nil {
		return err
	}

	root := args[0]

	root, err = filepath.Abs(root)
	if err != nil {
		return err
	}

	rootInfo, err := os.Stat(root)
	if err != nil {
		return err
	}

	var fileSystem fs.FS

	if rootInfo.IsDir() {
		fileSystem = os.DirFS(root)
	} else {
		fileSystem = os.DirFS(filepath.Dir(root))
		fileSystem, err = fs.Sub(fileSystem, filepath.Base(root))
		if err != nil {
			return err
		}
	}

	var excludePattern *regexp.Regexp

	if exclude != "" {
		excludePattern, err = regexp.Compile(exclude)
		if err != nil {
			return err
		}
	}

	if uploads {
		if uploadsDir == "" {
			uploadsDir = os.TempDir()
		}

		uploadsDirAbs, err := filepath.Abs(uploadsDir)
		if err != nil {
			return err
		}

		uploadsDir = uploadsDirAbs

		_, err = os.Stat(uploadsDir)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				return err
			}

			err = os.MkdirAll(uploadsDir, 0755)
			if err != nil {
				return err
			}
		}
	}

	serveTLS := tlsCert != "" && tlsKey != ""

	if host == "" {
		host = "0.0.0.0"
	}

	if port == 0 {
		if serveTLS {
			port = 443
		} else {
			port = 80
		}
	}

	address := net.JoinHostPort(host, strconv.FormatUint(port, 10))

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}

	scheme := "http"

	if serveTLS {
		scheme = "https"

		certificate, err := tls.LoadX509KeyPair(tlsCert, tlsKey)
		if err != nil {
			return err
		}

		listener = tls.NewListener(listener, &tls.Config{
			Certificates: []tls.Certificate{certificate},
		})
	}

	url := &url.URL{
		Scheme: scheme,
		Host:   strings.ReplaceAll(address, "0.0.0.0", "localhost"),
	}

	controller := files.NewController(fileSystem, files.ControllerConfig{
		FilesURL:         "/files",
		ExcludePattern:   excludePattern,
		Uploads:          uploads,
		UploadsDir:       uploadsDir,
		UploadsTimestamp: uploadsTimestamp,
		Version:          Version(),
	})

	mux := http.NewServeMux()

	printfln("")
	printfln(`   __________  ________  ______   _____ `)
	printfln(`  / __  / __ \/ ___/ _ \/ ___/ | / / _ \`)
	printfln(` / /_/ / /_/ (__  )  __/ /   | |/ /  __/`)
	printfln(` \__, /\____/____/\___/_/    |___/\___/ `)
	printfln(`/____/                                  `)
	printfln("")
	printfln("")
	printfln("Starting HTTP file server")
	printfln("")
	printfln("Config:")

	err = printConfigs([]config{
		{
			key:   "Root",
			value: root,
		},
		{
			key:   "Host",
			value: host,
		},
		{
			key:   "Port",
			value: port,
		},
		{
			key:      "Exclude Pattern",
			value:    excludePattern,
			disabled: excludePattern == nil,
		},
		{
			key:      "Uploads Dir",
			value:    uploadsDir,
			disabled: !uploads,
		},
		{
			key:   "Log Level",
			value: logLevel,
		},
		{
			key:   "Log Format",
			value: logFormat,
		},
		{
			key:      "TLS Cert",
			value:    tlsCert,
			disabled: !serveTLS,
		},
		{
			key:      "TLS Key",
			value:    tlsKey,
			disabled: !serveTLS,
		},
	})
	if err != nil {
		return err
	}

	printfln("")
	printfln("Routes:")

	err = registerRoutes(mux, []route{
		{
			pattern:     "/",
			description: "Redirect /files",
			handler:     redirect("/files", http.StatusMovedPermanently),
		},
		{
			pattern:     "GET /files",
			description: "List Files",
			handler:     controller.ListFiles(),
		},
		{
			pattern:     "GET /files/{file...}",
			description: "List Files",
			handler:     controller.ListFiles(),
			disabled:    !rootInfo.IsDir(),
		},
		{
			pattern:     "POST /files",
			description: "Upload File",
			handler:     controller.UploadFile(),
			disabled:    !uploads,
		},
		{
			pattern:     "GET /health",
			description: "Health Check",
			handler:     health(),
		},
	})
	if err != nil {
		return err
	}

	printfln("")
	printfln("Serving files at %s", url)
	printfln("")
	printfln("Ready to accept connections")
	printfln("")

	return http.Serve(listener, logging.LogRequests(mux))
}
