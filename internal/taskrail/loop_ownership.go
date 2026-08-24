package taskrail

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/tessariq/taskrail/internal/repolock"
)

var loopChildEnvironmentNames = []string{
	"TASKRAIL",
	"TASKRAIL_EXECUTABLE_SHA256",
	"TASKRAIL_DELEGATION_ID",
	"TASKRAIL_DELEGATION_TOKEN",
}

var loopExecutablePath = os.Executable

type loopStagedExecutable struct {
	Path      string
	guardPath string
	SHA256    string
	info      os.FileInfo
}

type loopOwnership struct {
	lock       *repolock.Lock
	executable loopStagedExecutable
	invocation string
}

// loopChildIdentity is the immutable executable identity plus the per-task
// delegation secret a later launcher passes only to its selected child.
type loopChildIdentity struct {
	Executable   string
	SHA256       string
	InvocationID string
	Token        string
}

// beginLoopOwnership takes the loop's one long-lived writer lock before it
// creates its private executable copy. Later loop stages perform semantic
// preflight and child execution while this ownership remains live.
func (s *Service) beginLoopOwnership(ctx context.Context) (*loopOwnership, error) {
	for _, name := range loopChildEnvironmentNames {
		if _, present := os.LookupEnv(name); present {
			return nil, fmt.Errorf("loop refuses inherited %s", name)
		}
	}
	executable, err := loopExecutablePath()
	if err != nil {
		return nil, fmt.Errorf("resolve running executable: %w", err)
	}
	invocation, err := randomLoopInvocationID()
	if err != nil {
		return nil, err
	}
	lock, err := repolock.Acquire(ctx, repolock.Request{
		Repository:    s.paths.LockRepository(),
		Command:       "loop",
		TransactionID: invocation,
		Capability:    repolock.Capability{Commands: []string{"loop"}},
	})
	if err != nil {
		return nil, err
	}
	staged, err := stageLoopExecutable(s.paths.GitCommonDir, executable)
	if err != nil {
		return nil, errors.Join(err, lock.Release())
	}
	return &loopOwnership{lock: lock, executable: staged, invocation: invocation}, nil
}

// delegate rotates the child-only grant for the next selected task without
// releasing the loop lock or changing the staged executable identity. The
// caller must wait for the previous delegated child before rotating it.
func (o *loopOwnership) delegate(grant repolock.Capability) (loopChildIdentity, error) {
	if err := o.executable.validate(); err != nil {
		return loopChildIdentity{}, err
	}
	delegation, err := o.lock.Delegate(o.executable.Path, o.executable.SHA256, grant)
	if err != nil {
		return loopChildIdentity{}, err
	}
	return loopChildIdentity{
		Executable: o.executable.Path, SHA256: o.executable.SHA256,
		InvocationID: o.invocation, Token: delegation.Token,
	}, nil
}

// close removes only the file this ownership created while the lock is still
// held, then releases the lock for a later writer or loop invocation.
func (o *loopOwnership) close() error {
	if err := o.executable.remove(); err != nil {
		return err
	}
	return o.lock.Release()
}

func randomLoopInvocationID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate loop invocation id: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

func stageLoopExecutable(gitCommonDir, source string) (loopStagedExecutable, error) {
	if !filepath.IsAbs(gitCommonDir) {
		return loopStagedExecutable{}, errors.New("loop requires an absolute Git common directory")
	}
	resolved, err := filepath.EvalSymlinks(source)
	if err != nil {
		return loopStagedExecutable{}, fmt.Errorf("resolve running executable %s: %w", source, err)
	}
	input, err := os.Open(resolved)
	if err != nil {
		return loopStagedExecutable{}, fmt.Errorf("open running executable %s: %w", resolved, err)
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return loopStagedExecutable{}, fmt.Errorf("stat running executable %s: %w", resolved, err)
	}
	if !info.Mode().IsRegular() {
		return loopStagedExecutable{}, fmt.Errorf("running executable %s is not a regular file", resolved)
	}
	expected, err := executableFileDigest(input)
	if err != nil {
		return loopStagedExecutable{}, fmt.Errorf("hash running executable %s: %w", resolved, err)
	}
	if _, err := input.Seek(0, io.SeekStart); err != nil {
		return loopStagedExecutable{}, fmt.Errorf("rewind running executable %s: %w", resolved, err)
	}

	dir := filepath.Join(gitCommonDir, "taskrail", "executables")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return loopStagedExecutable{}, fmt.Errorf("create loop executable directory: %w", err)
	}
	path := filepath.Join(dir, expected)
	output, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o700)
	if err != nil {
		return loopStagedExecutable{}, fmt.Errorf("stage loop executable %s: %w", path, err)
	}
	remove := func(err error) (loopStagedExecutable, error) {
		_ = output.Close()
		_ = os.Remove(path)
		return loopStagedExecutable{}, err
	}
	digest := sha256.New()
	if _, err := io.Copy(io.MultiWriter(output, digest), input); err != nil {
		return remove(fmt.Errorf("copy running executable: %w", err))
	}
	if err := output.Sync(); err != nil {
		return remove(fmt.Errorf("sync staged executable: %w", err))
	}
	if err := output.Close(); err != nil {
		_ = os.Remove(path)
		return loopStagedExecutable{}, fmt.Errorf("close staged executable: %w", err)
	}
	actual := hex.EncodeToString(digest.Sum(nil))
	if actual != expected {
		_ = os.Remove(path)
		return loopStagedExecutable{}, errors.New("running executable changed while it was staged")
	}
	verified, err := repolock.ExecutableDigest(path)
	if err != nil {
		_ = os.Remove(path)
		return loopStagedExecutable{}, fmt.Errorf("verify staged executable: %w", err)
	}
	if verified != expected {
		_ = os.Remove(path)
		return loopStagedExecutable{}, errors.New("staged executable digest changed after copy")
	}
	stagedInfo, err := os.Lstat(path)
	if err != nil || !stagedInfo.Mode().IsRegular() {
		_ = os.Remove(path)
		return loopStagedExecutable{}, fmt.Errorf("inspect staged executable: %w", err)
	}
	guardPath, err := loopStagingGuard(path)
	if err != nil {
		_ = os.Remove(path)
		return loopStagedExecutable{}, err
	}
	guardInfo, err := os.Lstat(guardPath)
	if err != nil || !guardInfo.Mode().IsRegular() || !os.SameFile(stagedInfo, guardInfo) {
		_ = os.Remove(guardPath)
		_ = os.Remove(path)
		return loopStagedExecutable{}, fmt.Errorf("inspect staged executable guard: %w", err)
	}
	return loopStagedExecutable{Path: path, guardPath: guardPath, SHA256: expected, info: stagedInfo}, nil
}

func executableFileDigest(file *os.File) (string, error) {
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

func loopStagingGuard(path string) (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate staged executable guard: %w", err)
	}
	guard := path + ".owner-" + hex.EncodeToString(raw[:])
	if err := os.Link(path, guard); err != nil {
		return "", fmt.Errorf("link staged executable guard: %w", err)
	}
	return guard, nil
}

func (e loopStagedExecutable) remove() error {
	if err := e.validate(); err != nil {
		return err
	}
	if err := os.Remove(e.Path); err != nil {
		return fmt.Errorf("remove staged executable %s: %w", e.Path, err)
	}
	if err := os.Remove(e.guardPath); err != nil {
		return fmt.Errorf("remove staged executable guard %s: %w", e.guardPath, err)
	}
	return nil
}

func (e loopStagedExecutable) validate() error {
	info, err := os.Lstat(e.Path)
	if err != nil {
		return fmt.Errorf("inspect staged executable: %w", err)
	}
	if !info.Mode().IsRegular() || !os.SameFile(info, e.info) {
		return fmt.Errorf("refuse changed staged executable %s", e.Path)
	}
	guard, err := os.Lstat(e.guardPath)
	if err != nil || !guard.Mode().IsRegular() || !os.SameFile(info, guard) || !os.SameFile(guard, e.info) {
		return fmt.Errorf("refuse changed staged executable guard %s", e.guardPath)
	}
	digest, err := repolock.ExecutableDigest(e.Path)
	if err != nil || digest != e.SHA256 {
		return fmt.Errorf("refuse changed staged executable %s", e.Path)
	}
	return nil
}
