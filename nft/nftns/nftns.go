/*
 * This file is part of the go-nft project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package nftns

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	nftconfig "github.com/networkplumbing/go-nft/nft/config"
	"github.com/networkplumbing/go-nft/nft/schema"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	cmdFile    = "-f"
	cmdJSON    = "-j"
	cmdList    = "list"
	cmdRuleset = "ruleset"
	cmdStdin   = "-"
)

var (
	NSEnterBinPath = "nsenter"
	NFTBinPath     = "nft"
)

var Logger zerolog.Logger

func init() {
	Logger = log.Logger
	NSEnterBinPath, _ = exec.LookPath("nsenter")
	NFTBinPath, _ = exec.LookPath("nft")
}

type Config struct {
	nftconfig.Config
	NetNSPath string `json:"-"`
}

// New returns a new nftables config structure.
func New(netNSPath string) (*Config, error) {
	c := &Config{
		NetNSPath: netNSPath,
	}

	// hmm.  concurrency issues?
	if NSEnterBinPath == "" {
		path, err := exec.LookPath("nsenter")
		if err != nil {
			return nil, err
		}
		NSEnterBinPath = path
	}

	c.Nftables = []schema.Nftable{}
	return c, nil
}

// ReadConfig loads the nftables configuration from the system and
// returns it as a nftables config structure.
// The system is expected to have the `nft` executable deployed and nftables enabled in the kernel.
func ReadConfig(netNSPath string) (*Config, error) {
	stdout, err := execCommand(netNSPath, nil, cmdJSON, cmdList, cmdRuleset)
	if err != nil {
		return nil, err
	}

	config, err := New(netNSPath)
	if err != nil {
		return nil, err
	}
	if err = config.FromJSON(stdout.Bytes()); err != nil {
		return nil, fmt.Errorf("failed to list ruleset: %v", err)
	}

	return config, nil
}

// ApplyConfig applies the given nftables config on the system.
// The system is expected to have the `nft` executable deployed and nftables enabled in the kernel.
func ApplyConfig(c *Config) error {
	data, err := c.ToJSON()
	if err != nil {
		return err
	}

	if _, err := execCommand(c.NetNSPath, data, cmdJSON, cmdFile, cmdStdin); err != nil {
		return err
	}

	return nil
}

func execCommand(netNSPath string, input []byte, args ...string) (*bytes.Buffer, error) {
	fullArgs := append([]string{
		fmt.Sprintf("--net=%s", netNSPath),
		"--",
		NFTBinPath,
	}, args...)

	Logger.Trace().Msgf("Running nsenter command: %v %v", NSEnterBinPath, fullArgs)
	cmd := exec.Command(NSEnterBinPath, fullArgs...)

	var stdout, stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout

	if input != nil {
		var stdin bytes.Buffer
		stdin.Write(input)
		cmd.Stdin = &stdin
	}

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf(
			"failed to execute %s %s: %w stdin:'%s' stdout:'%s' stderr:'%s'",
			cmd.Path, strings.Join(cmd.Args, " "), err, string(input), stdout.String(), stderr.String(),
		)
	}

	return &stdout, nil
}
