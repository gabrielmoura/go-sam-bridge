// Package main provides the entry point for the SAM bridge server.
// The SAM bridge implements the SAMv3.3 protocol, enabling applications
// to communicate over the I2P anonymity network using text-based commands.
//
// Usage:
//
//	sam-bridge [flags]
//
// Flags:
//
//	-listen string     SAM listen address (default "127.0.0.1:7656")
//	-i2cp string       I2CP router address (default "127.0.0.1:7654")
//	-udp string        UDP datagram port (disabled by default)
//	-debug             Enable debug logging
//	-user string       I2CP username (optional)
//	-pass string       I2CP password (optional)
//	-version           Show version information
//	-help              Show help message
//
// Environment variables:
//
//	SAM_LISTEN    SAM listen address (overrides -listen)
//	I2CP_ADDR     I2CP router address (overrides -i2cp)
//	SAM_DEBUG     Enable debug logging (overrides -debug)
//
// See SAMv3.md for the complete SAM protocol specification.
package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/go-i2p/go-sam-bridge/lib/embedding"
	"github.com/go-i2p/go-sam-bridge/lib/i2cp"
	"github.com/go-i2p/logger"
)

var (
	// Version is set at build time via ldflags
	Version = "dev"

	// Build info
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func main() {
	cfg := parseFlags()

	// Configure logging
	log := logger.GetGoI2PLogger()
	if cfg.Debug {
		log.SetLevel(logger.DebugLevel)
	} else {
		log.SetLevel(logger.InfoLevel)
	}

	log.WithFields(logger.Fields{
		"pkg":       "main",
		"func":      "main",
		"version":   Version,
		"buildTime": BuildTime,
		"commit":    GitCommit,
	}).Info("Starting SAM bridge server")

	// Parse datagram port
	datagramPort := parseDatagramPort(cfg.UDPAddr)

	// Attempt I2CP connection; failure is non-fatal — the embedded router will be
	// started automatically by embedding.New() when port 7654 is free.
	var i2cpClient *i2cp.Client
	client, err := connectI2CP(cfg, log)
	if err != nil {
		log.WithFields(logger.Fields{"pkg": "main", "func": "main"}).WithError(err).Warn("No external I2P router available; using embedded router fallback")
		log.WithFields(logger.Fields{"pkg": "main", "func": "main"}).Info("STREAM/DATAGRAM/RAW sessions will be activated once the embedded router is ready")
	} else {
		i2cpClient = client
		defer i2cpClient.Close()
		log.WithFields(logger.Fields{"pkg": "main", "func": "main", "version": i2cpClient.RouterVersion()}).Info("Connected to I2P router")
	}

	// Build bridge options. embedding wires a provider for an external client,
	// or starts and manages a router plus client when no client was supplied.
	opts := []embedding.Option{
		embedding.WithListenAddr(cfg.ListenAddr),
		embedding.WithI2CPAddr(cfg.I2CPAddr),
		embedding.WithDatagramPort(datagramPort),
		embedding.WithLogger(log),
		embedding.WithDebug(cfg.Debug),
	}
	if i2cpClient != nil {
		opts = append(opts, embedding.WithI2CPClient(i2cpClient))
	}

	// Create bridge with embedding API
	bridge, err := embedding.New(opts...)
	if err != nil {
		log.WithFields(logger.Fields{"pkg": "main", "func": "main"}).WithError(err).Error("Failed to create bridge")
		os.Exit(1)
	}

	// Start bridge
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := bridge.Start(ctx); err != nil {
		log.WithFields(logger.Fields{"pkg": "main", "func": "main"}).WithError(err).Error("Failed to start bridge")
		os.Exit(1)
	}

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.WithFields(logger.Fields{"pkg": "main", "func": "main"}).Info("Received shutdown signal")
	bridge.Stop(context.Background())
}

// Config holds command-line configuration.
type Config struct {
	ListenAddr string
	I2CPAddr   string
	UDPAddr    string
	Debug      bool
	Username   string
	Password   string
}

func parseFlags() *Config {
	cfg := &Config{}

	flag.StringVar(&cfg.ListenAddr, "listen", embedding.DefaultListenAddr, "SAM listen address")
	flag.StringVar(&cfg.I2CPAddr, "i2cp", "127.0.0.1:7654", "I2CP router address")
	flag.StringVar(&cfg.UDPAddr, "udp", "", "UDP datagram port (disabled by default)")
	flag.BoolVar(&cfg.Debug, "debug", false, "Enable debug logging")
	flag.StringVar(&cfg.Username, "user", "", "I2CP username (optional)")
	flag.StringVar(&cfg.Password, "pass", "", "I2CP password (optional)")

	showVersion := flag.Bool("version", false, "Show version information")
	showHelp := flag.Bool("help", false, "Show help message")

	flag.Parse()

	if *showVersion {
		fmt.Printf("sam-bridge %s\n", Version)
		fmt.Printf("Build time: %s\n", BuildTime)
		fmt.Printf("Git commit: %s\n", GitCommit)
		os.Exit(0)
	}

	if *showHelp {
		fmt.Println("SAM Bridge - SAMv3.3 Protocol Bridge for I2P")
		fmt.Println()
		fmt.Println("Usage: sam-bridge [flags]")
		fmt.Println()
		fmt.Println("Flags:")
		flag.PrintDefaults()
		fmt.Println()
		fmt.Println("Environment variables:")
		fmt.Println("  SAM_LISTEN    SAM listen address (overrides -listen)")
		fmt.Println("  I2CP_ADDR     I2CP router address (overrides -i2cp)")
		fmt.Println("  SAM_DEBUG     Enable debug logging (overrides -debug)")
		os.Exit(0)
	}

	// Override with environment variables
	if env := os.Getenv("SAM_LISTEN"); env != "" {
		cfg.ListenAddr = env
	}
	if env := os.Getenv("I2CP_ADDR"); env != "" {
		cfg.I2CPAddr = env
	}
	if os.Getenv("SAM_DEBUG") != "" {
		cfg.Debug = true
	}

	return cfg
}

func connectI2CP(cfg *Config, log *logger.Logger) (*i2cp.Client, error) {
	i2cpConfig := &i2cp.ClientConfig{
		RouterAddr: cfg.I2CPAddr,
		Username:   cfg.Username,
		Password:   cfg.Password,
	}

	client := i2cp.NewClient(i2cpConfig)
	ctx := context.Background()

	log.WithFields(logger.Fields{"pkg": "main", "func": "connectI2CP", "addr": cfg.I2CPAddr}).Info("Connecting to I2P router")
	if err := client.Connect(ctx); err != nil {
		return nil, err
	}

	return client, nil
}

func parseDatagramPort(addr string) int {
	if addr == "" {
		return embedding.DefaultDatagramPort
	}
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		portStr = addr
	}
	if port, err := strconv.Atoi(portStr); err == nil {
		return port
	}
	return embedding.DefaultDatagramPort
}
