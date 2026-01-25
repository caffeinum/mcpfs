package fs

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/winfsp/cgofuse/fuse"

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

	cgoFS := NewCgoFS(cfg, p)
	host := fuse.NewFileSystemHost(cgoFS)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nshutting down...")
		host.Unmount()
	}()

	fmt.Printf("mounting at %s\n", opts.Mountpoint)
	fmt.Println("press ctrl+c to unmount")

	// fuse-t uses NFS internally, no kernel extension needed
	mountArgs := []string{opts.Mountpoint}

	ok := host.Mount("", mountArgs)

	p.Close()

	if !ok {
		return fmt.Errorf("mount failed")
	}
	return nil
}

func Unmount(mountpoint string) error {
	return syscall.Unmount(mountpoint, 0)
}
