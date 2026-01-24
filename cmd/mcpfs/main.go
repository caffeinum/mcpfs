package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
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
	mountCmd.Flags().BoolP("foreground", "f", false, "run in foreground")
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
	foreground, _ := cmd.Flags().GetBool("foreground")
	configDir, _ := cmd.Flags().GetString("config")

	fmt.Printf("mounting mcpfs at %s (foreground=%v, config=%s)\n", mountpoint, foreground, configDir)
	// TODO: implement
	return nil
}

func runUmount(cmd *cobra.Command, args []string) error {
	mountpoint := args[0]
	fmt.Printf("unmounting mcpfs at %s\n", mountpoint)
	// TODO: implement
	return nil
}

func runAdd(cmd *cobra.Command, args []string) error {
	name := args[0]
	url, _ := cmd.Flags().GetString("url")

	if url != "" {
		fmt.Printf("adding http server %s at %s\n", name, url)
	} else if len(args) > 1 {
		fmt.Printf("adding stdio server %s: %v\n", name, args[1:])
	} else {
		return fmt.Errorf("must provide --url or command after --")
	}
	// TODO: implement
	return nil
}

func runAuth(cmd *cobra.Command, args []string) error {
	server := args[0]
	token := args[1]
	fmt.Printf("storing auth token for %s (length=%d)\n", server, len(token))
	// TODO: implement
	return nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	fmt.Println("server status:")
	fmt.Println("  (no servers connected)")
	// TODO: implement
	return nil
}

func runList(cmd *cobra.Command, args []string) error {
	fmt.Println("configured servers:")
	fmt.Println("  (no servers configured)")
	// TODO: implement
	return nil
}
