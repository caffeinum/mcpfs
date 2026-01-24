package fs

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	gofuse "github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"

	"github.com/caffeinum/mcpfs/internal/config"
	"github.com/caffeinum/mcpfs/internal/pool"
)

type MountOptions struct {
	Mountpoint string
	ConfigDir  string
	Foreground bool
}

func Mount(opts MountOptions) error {
	cfg, err := config.Load(opts.ConfigDir)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	p := pool.New(pool.PoolConfig{
		Config: cfg,
	})

	mcpfs := New(cfg, p)
	root := &RootNode{mcpfs: mcpfs}

	mountOpts := &gofuse.Options{
		MountOptions: fuse.MountOptions{
			Name:   "mcpfs",
			FsName: "mcpfs",
		},
	}

	server, err := gofuse.Mount(opts.Mountpoint, root, mountOpts)
	if err != nil {
		p.Close()
		return fmt.Errorf("mount: %w", err)
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nshutting down...")
		server.Unmount()
	}()

	fmt.Printf("mounted at %s\n", opts.Mountpoint)
	fmt.Println("press ctrl+c to unmount")

	server.Wait()
	p.Close()

	return nil
}

func Unmount(mountpoint string) error {
	return syscall.Unmount(mountpoint, 0)
}
