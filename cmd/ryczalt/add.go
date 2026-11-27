package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// cmdAdd puts documents (the invoice PDF, a contract, a ZUS receipt) into
// a month folder: the newest invoice's by default.
func cmdAdd(args []string) error {
	flags := flag.NewFlagSet("add", flag.ContinueOnError)
	cfgFlag := flags.String("c", "", "config file")
	monthFlag := flags.String(
		"month",
		"",
		"month `FOLDER`, e.g. 2026-12 (default: the newest invoice's)",
	)

	var move bool
	flags.BoolVar(&move, "move", false, "move the files instead of copying them")
	flags.BoolVar(&move, "m", false, "short for --move")

	flags.Usage = func() {
		fmt.Fprintln(
			os.Stderr,
			"usage: ryczalt add [-c CONFIG] [--month YYYY-MM] [--move|-m] FILE...",
		)
	}

	if err := parseFlags(flags, args); err != nil {
		return exitError{2, err}
	}

	if flags.NArg() == 0 {
		flags.Usage()
		return exitError{2, errors.New("no files given")}
	}

	c, err := loadConfig(*cfgFlag)
	if err != nil {
		return err
	}

	folder := *monthFlag
	if folder == "" {
		invs, _, err := readArchive(c.archive, c.nip)
		if err != nil {
			return err
		}
		if len(invs) == 0 {
			return exitError{2, errors.New("no invoice in the archive yet; give --month")}
		}
		folder = invs[len(invs)-1].issued.Format("2006-01")
	}

	if !folderName.MatchString(folder) {
		return exitError{2, fmt.Errorf("--month %q is not like 2026-12", folder)}
	}
	dir := filepath.Join(c.archive, folder)

	// check everything first, so nothing is half done
	for _, src := range flags.Args() {
		st, err := os.Stat(src)
		if err != nil {
			return err
		}
		if !st.Mode().IsRegular() {
			return fmt.Errorf("%s is not a file", src)
		}

		if _, err := os.Stat(filepath.Join(dir, filepath.Base(src))); err == nil {
			return fmt.Errorf(
				"%s already exists, leaving it alone",
				filepath.Join(dir, filepath.Base(src)),
			)
		}
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	w := newReport(os.Stdout)
	verb := "copied"
	if move {
		verb = "moved"
	}

	for _, src := range flags.Args() {
		dst := filepath.Join(dir, filepath.Base(src))
		if err := copyFile(src, dst); err != nil {
			return err
		}

		if move {
			if err := os.Remove(src); err != nil {
				return fmt.Errorf("copied to %s, but could not remove %s: %w", dst, src, err)
			}
		}

		w.printf("%s %s %s\n", tag(w, green, "OK"), verb, dst)
	}

	return w.flush()
}

// copyFile copies src to a new dst (never over an existing one), keeps the
// modification time, and checks the size before it counts as done.
func copyFile(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }() // read only

	st, err := in.Stat()
	if err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("%s already exists, leaving it alone", dst)
	}
	if err != nil {
		return err
	}

	n, err := io.Copy(out, in)
	if cerr := out.Close(); err == nil {
		err = cerr
	}

	if err == nil && n != st.Size() {
		err = fmt.Errorf("copied %d of %d bytes", n, st.Size())
	}
	if err != nil {
		_ = os.Remove(dst) // a partial copy is worse than none
		return fmt.Errorf("%s: %w", dst, err)
	}

	return os.Chtimes(dst, time.Now(), st.ModTime())
}
