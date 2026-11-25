package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"csp-unzip/internal/sevenzip"
)

const (
	enableLineInput = 0x0002
	enableEchoInput = 0x0004
)

var setConsoleMode = syscall.NewLazyDLL("kernel32.dll").NewProc("SetConsoleMode")

type console struct {
	reader      *bufio.Reader
	handle      syscall.Handle
	interactive bool
}

func (c *console) setMode(mode uint32) error {
	ok, _, err := setConsoleMode.Call(uintptr(c.handle), uintptr(mode))
	if ok == 0 {
		return err
	}
	return nil
}

func (c *console) password(ctx context.Context, prompt string) (password []byte, err error) {
	if c.interactive {
		var mode uint32
		if err := syscall.GetConsoleMode(c.handle, &mode); err != nil {
			return nil, err
		}
		visibleMode := mode | enableLineInput | enableEchoInput
		if mode != visibleMode {
			if err := c.setMode(visibleMode); err != nil {
				return nil, fmt.Errorf("无法开启密码输入回显：%w", err)
			}
			defer func() {
				if restoreErr := c.setMode(mode); restoreErr != nil && err == nil {
					clear(password)
					password, err = nil, restoreErr
				}
			}()
		}
	}
	fmt.Print(prompt + "：")
	// A console read can still be pending when the user presses Ctrl+C.
	// Let the caller restore console mode immediately on cancellation.
	type result struct {
		line []byte
		err  error
	}
	done := make(chan result, 1)
	go func() {
		line, err := c.reader.ReadBytes('\n')
		done <- result{line, err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-done:
		if r.err != nil && !(errors.Is(r.err, io.EOF) && len(r.line) > 0) {
			clear(r.line)
			if errors.Is(r.err, io.EOF) {
				return nil, errors.New("密码输入已结束")
			}
			return nil, r.err
		}
		return bytes.TrimSuffix(bytes.TrimSuffix(r.line, []byte("\n")), []byte("\r")), nil
	}
}

func main() {
	os.Exit(mainCode())
}

func mainCode() int {
	if len(os.Args) == 2 && os.Args[1] == "--license" {
		fmt.Print(sevenzip.Notice())
		return 0
	}
	if len(os.Args) != 1 {
		fmt.Fprintf(os.Stderr, "查看第三方许可请运行 %s --license\n", filepath.Base(os.Args[0]))
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	handle := syscall.Handle(os.Stdin.Fd())
	var mode uint32
	c := console{bufio.NewReader(os.Stdin), handle, syscall.GetConsoleMode(handle, &mode) == nil}
	fmt.Println("CSP 双层解压工具")
	exe, err := os.Executable()
	if err == nil {
		err = run(ctx, filepath.Dir(exe), func(prompt string) ([]byte, error) {
			return c.password(ctx, prompt)
		}, os.Stdout, sevenzip.Materialize)
	}
	if ctx.Err() != nil {
		fmt.Fprintln(os.Stderr, "操作已取消。")
		if err != nil && err != ctx.Err() {
			fmt.Fprintln(os.Stderr, err)
		}
		return 1
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "操作失败：", err)
	}
	if c.interactive {
		// Restore the default Ctrl+C action while waiting at the final prompt.
		stop()
		fmt.Print("按回车键退出……")
		_, _ = c.reader.ReadBytes('\n')
	}
	if err != nil {
		return 1
	}
	return 0
}
