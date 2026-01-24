package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/caffeinum/mcpfs/internal/config"
	"github.com/caffeinum/mcpfs/internal/fs"
)

var version = "dev"

func main() {
	rootCmd := &cobra.Command{
		Use:     "mcpfs",
		Short:   "mount mcp servers as a fuse filesystem",
		Version: version,
	}

	mountCmd := &cobra.Command{
		Use:   "mount <mountpoint>",
		Short: "mount the mcp filesystem",
		Args:  cobra.ExactArgs(1),
		RunE:  runMount,
	}
	mountCmd.Flags().BoolP("foreground", "f", false, "run in foreground (always true for now)")
	mountCmd.Flags().String("config", "", "config directory (default: ~/.mcp/.config)")

	umountCmd := &cobra.Command{
		Use:   "umount <mountpoint>",
		Short: "unmount the mcp filesystem",
		Args:  cobra.ExactArgs(1),
		RunE:  runUmount,
	}

	addCmd := &cobra.Command{
		Use:   "add <name> [-- <command>...]",
		Short: "add an mcp server to config",
		Args:  cobra.MinimumNArgs(1),
		RunE:  runAdd,
	}
	addCmd.Flags().String("url", "", "http server url (for http transport)")

	authCmd := &cobra.Command{
		Use:   "auth <server> <token>",
		Short: "store auth token for a server",
		Args:  cobra.ExactArgs(2),
		RunE:  runAuth,
	}

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "show server connection status",
		RunE:  runStatus,
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "list configured servers",
		RunE:  runList,
	}

	rootCmd.AddCommand(mountCmd, umountCmd, addCmd, authCmd, statusCmd, listCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runMount(cmd *cobra.Command, args []string) error {
	mountpoint := args[0]
	configDir, _ := cmd.Flags().GetString("config")

	if err := os.MkdirAll(mountpoint, 0755); err != nil {
		return fmt.Errorf("create mountpoint: %w", err)
	}

	return fs.Mount(fs.MountOptions{
		Mountpoint: mountpoint,
		ConfigDir:  configDir,
		Foreground: true,
	})
}

func runUmount(cmd *cobra.Command, args []string) error {
	mountpoint := args[0]
	if err := fs.Unmount(mountpoint); err != nil {
		return fmt.Errorf("unmount: %w", err)
	}
	fmt.Printf("unmounted %s\n", mountpoint)
	return nil
}

func runAdd(cmd *cobra.Command, args []string) error {
	name := args[0]
	url, _ := cmd.Flags().GetString("url")
	configDir, _ := cmd.Flags().GetString("config")

	cfg, err := config.Load(configDir)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if url != "" {
		cfg.AddHTTPServer(name, url, map[string]string{
			"Authorization": "Bearer ${auth.token}",
		})
		fmt.Printf("added http server: %s\n", name)
	} else if len(args) > 1 {
		command := args[1]
		cmdArgs := args[2:]
		cfg.AddStdioServer(name, command, cmdArgs, nil)
		fmt.Printf("added stdio server: %s\n", name)
	} else {
		return fmt.Errorf("must provide --url or command after --")
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	return nil
}

func runAuth(cmd *cobra.Command, args []string) error {
	server := args[0]
	token := args[1]
	configDir, _ := cmd.Flags().GetString("config")

	if err := config.SaveToken(configDir, server, token); err != nil {
		return fmt.Errorf("save token: %w", err)
	}

	fmt.Printf("saved auth token for %s\n", server)
	return nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	fmt.Println("server status:")
	fmt.Println("  (mount filesystem first to see connection status)")
	return nil
}

func runList(cmd *cobra.Command, args []string) error {
	configDir, _ := cmd.Flags().GetString("config")

	cfg, err := config.Load(configDir)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if len(cfg.Servers) == 0 {
		fmt.Println("no servers configured")
		fmt.Println("use 'mcpfs add <name> -- <command>' to add a stdio server")
		fmt.Println("or 'mcpfs add <name> --url <url>' to add an http server")
		return nil
	}

	fmt.Println("configured servers:")
	for name, srv := range cfg.Servers {
		switch srv.Transport {
		case config.TransportStdio:
			fmt.Printf("  %s (stdio): %s %v\n", name, srv.Command, srv.Args)
		case config.TransportHTTP:
			fmt.Printf("  %s (http): %s\n", name, srv.URL)
		}
	}

	return nil
}
