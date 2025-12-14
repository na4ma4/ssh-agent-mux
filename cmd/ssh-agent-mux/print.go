package main

import (
	"fmt"
	"os"

	"github.com/na4ma4/ssh-agent-mux/api"
	"github.com/spf13/viper"
)

func PrintConfig(cfg *api.Config) {
	fmt.Fprintln(os.Stdout, "## SSH Agent Multiplexer started")
	fmt.Fprintf(os.Stdout, "SSH_AUTH_SOCK=%s; export SSH_AUTH_SOCK\n", cfg.GetSocketPath())
	fmt.Fprintf(os.Stdout, "SSH_AGENT_PID=%d; export SSH_AGENT_PID\n", cfg.GetPid())
	if !viper.GetBool("quiet") {
		fmt.Fprintf(os.Stdout, "echo Agent pid %d\n", cfg.GetPid())
	}
}
