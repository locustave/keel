package session

import (
	"os"
	"os/exec"
	"os/user"
	"strings"
)

// ---------------------------------------------------------------------------
// Identity types
// ---------------------------------------------------------------------------

// Identity represents a person, agent, or system.
type Identity struct {
	UserID string  // e.g. "local:joeymiller50"
	Name   string  // e.g. "Joey Miller"
	Email  *string // may be nil if unavailable
	Source string  // "cli_arg | local_config | git_config | env | os_user | unknown"
}

// Actor represents the entity that caused an event.
type Actor struct {
	ActorType string  `json:"actor_type"` // human | agent | harness | tool | hook | automation | system | unknown
	ActorName string  `json:"actor_name"`
	ActorID   string  `json:"actor_id"`
}

// Implementer represents who is currently implementing the phase.
type Implementer struct {
	ImplementerType string  `json:"implementer_type"` // human | agent | automation | mixed | unknown
	ImplementerName *string `json:"implementer_name"`
	ImplementerID   *string `json:"implementer_id"`
}

// CommandUser represents the human or process that invoked a command.
type CommandUser struct {
	UserID string  `json:"user_id"`
	Name   string  `json:"name"`
	Email  *string `json:"email"`
}

// ---------------------------------------------------------------------------
// Well-known actors
// ---------------------------------------------------------------------------

// HarnessCLIActor is the actor used for events emitted by the harness binary.
var HarnessCLIActor = Actor{
	ActorType: "keel",
	ActorName: "keel-cli",
	ActorID:   "local:keel-cli",
}

// UnknownImplementer is used when no implementer has been specified.
var UnknownImplementer = Implementer{
	ImplementerType: "unknown",
	ImplementerName: nil,
	ImplementerID:   nil,
}

// ---------------------------------------------------------------------------
// IdentityResolveOptions
// ---------------------------------------------------------------------------

// ResolveOptions holds explicit values that override auto-detection.
type ResolveOptions struct {
	// Explicit values from CLI flags.
	Name  string
	Email string
	// Config loaded from .agent/keel/config.yaml (optional).
	Config *UserConfig
}

// ResolveIdentity detects the session owner using the detection order:
//  1. Explicit CLI flags (ResolveOptions.Name / Email)
//  2. .agent/keel/config.yaml
//  3. git config user.name / user.email
//  4. Environment variables (GIT_AUTHOR_NAME, GIT_COMMITTER_NAME, USER, USERNAME)
//  5. OS username
//  6. "unknown"
func ResolveIdentity(opts ResolveOptions) Identity {
	// 1. Explicit CLI flags.
	if opts.Name != "" {
		return identityFromName(opts.Name, opts.Email, "cli_arg")
	}

	// 2. Config file.
	if opts.Config != nil {
		if opts.Config.Name != nil && *opts.Config.Name != "" {
			email := ""
			if opts.Config.Email != nil {
				email = *opts.Config.Email
			}
			return identityFromName(*opts.Config.Name, email, "local_config")
		}
	}

	// 3. git config.
	if name, email := gitConfigIdentity(); name != "" {
		return identityFromName(name, email, "git_config")
	}

	// 4. Environment variables.
	if name := envIdentity(); name != "" {
		return identityFromName(name, "", "env")
	}

	// 5. OS user.
	if u, err := user.Current(); err == nil && u.Username != "" {
		return identityFromName(u.Username, "", "os_user")
	}

	return Identity{
		UserID: "unknown",
		Name:   "unknown",
		Email:  nil,
		Source: "unknown",
	}
}

// ToCommandUser converts an Identity to a CommandUser for event records.
func (id Identity) ToCommandUser() CommandUser {
	return CommandUser{
		UserID: id.UserID,
		Name:   id.Name,
		Email:  id.Email,
	}
}

// ToActor converts an Identity to a human Actor.
func (id Identity) ToActor() Actor {
	return Actor{
		ActorType: "human",
		ActorName: id.Name,
		ActorID:   id.UserID,
	}
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func identityFromName(name, email, source string) Identity {
	id := Identity{
		UserID: "local:" + sanitizeUserID(name),
		Name:   name,
		Source: source,
	}
	if email != "" {
		id.Email = &email
	}
	return id
}

func sanitizeUserID(name string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), " ", ""))
}

func gitConfigIdentity() (name, email string) {
	if out, err := exec.Command("git", "config", "user.name").Output(); err == nil {
		name = strings.TrimSpace(string(out))
	}
	if out, err := exec.Command("git", "config", "user.email").Output(); err == nil {
		email = strings.TrimSpace(string(out))
	}
	return name, email
}

func envIdentity() string {
	for _, key := range []string{"GIT_AUTHOR_NAME", "GIT_COMMITTER_NAME", "USER", "USERNAME"} {
		if v := os.Getenv(key); v != "" {
			return v
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Implementer helpers
// ---------------------------------------------------------------------------

// MakeImplementer builds an Implementer from name and type strings.
func MakeImplementer(name, typ string) Implementer {
	if name == "" {
		return UnknownImplementer
	}
	if typ == "" {
		typ = "unknown"
	}
	id := typ + ":" + name
	return Implementer{
		ImplementerType: typ,
		ImplementerName: &name,
		ImplementerID:   &id,
	}
}

// MakeToolActor builds an Actor for a tool invocation.
func MakeToolActor(toolName string) Actor {
	return Actor{
		ActorType: "tool",
		ActorName: toolName,
		ActorID:   "tool:" + toolName,
	}
}

// MakeHookActor builds an Actor for a hook invocation.
func MakeHookActor(hookName string) Actor {
	return Actor{
		ActorType: "hook",
		ActorName: hookName,
		ActorID:   "hook:" + hookName,
	}
}
