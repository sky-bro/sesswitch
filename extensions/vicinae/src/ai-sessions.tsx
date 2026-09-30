import {
  Action,
  ActionPanel,
  Color,
  Form,
  Icon,
  Keyboard,
  List,
  LocalStorage,
  Toast,
  closeMainWindow,
  environment,
  getPreferenceValues,
  showToast,
  useNavigation,
} from "@vicinae/api";
import { execFile } from "node:child_process";
import { existsSync } from "node:fs";
import { homedir } from "node:os";
import { join } from "node:path";
import { promisify } from "node:util";
import { useCallback, useEffect, useState } from "react";

const execFileAsync = promisify(execFile);
type Preferences = { binaryPath?: string };

function expandHome(path: string): string {
  if (path === "~") return homedir();
  if (path.startsWith("~/")) return join(homedir(), path.slice(2));
  return path;
}

const preferences = getPreferenceValues<Preferences>();
const sesswitch = expandHome(preferences.binaryPath?.trim() || "~/.local/bin/sesswitch");
const organizationStorageKey = "session-organization";

type State = {
  kind: string;
  source: string;
  observed_at?: string;
};

type Location = {
  kind: string;
  tmux_pane?: string;
  needs_attention?: boolean;
};

type Session = {
  key: string;
  provider: string;
  title: string;
  cwd?: string;
  source?: string;
  updated_at: string;
  state: State;
  location?: Location;
  task?: { kind: string };
};

type Status = {
  label: string;
  color: Color;
};

type Organization = "focus" | "recent" | "project" | "agent";

type SessionGroup = {
  key: string;
  title: string;
  sessions: Session[];
};

function statusFor(session: Session): Status {
  if (session.task?.kind === "done") return { label: "Done", color: Color.Green };
  switch (session.state.kind) {
    case "needs_approval":
      return { label: "Action needed", color: Color.Orange };
    case "working":
      return { label: "Working", color: Color.Blue };
    case "turn_ended":
      return { label: "Ready to review", color: Color.Green };
    case "session_open":
      return { label: "Open", color: Color.Purple };
    case "interrupted":
      return { label: "Interrupted", color: Color.Red };
    case "error":
      return { label: "Error", color: Color.Red };
    case "idle":
      return { label: "Idle", color: Color.Yellow };
    case "closed":
      return { label: "Closed", color: Color.SecondaryText };
    default:
      return { label: "Saved", color: Color.SecondaryText };
  }
}

function providerIcon(provider: string) {
  if (provider === "codex") return join(environment.assetsPath, "codex.svg");
  if (provider === "claude") return join(environment.assetsPath, "claude.svg");
  return Icon.Terminal;
}

function providerName(provider: string): string {
  if (provider === "codex") return "Codex";
  if (provider === "claude") return "Claude";
  return provider;
}

function projectName(cwd?: string): string {
  if (!cwd) return "Unknown project";
  const parts = cwd.split("/").filter(Boolean);
  return parts.at(-1) ?? cwd;
}

function updatedAt(session: Session): number {
  const value = Date.parse(session.updated_at);
  return Number.isNaN(value) ? 0 : value;
}

function focusRank(session: Session): number {
  if (session.task?.kind === "done") return 8;
  switch (session.state.kind) {
    case "needs_approval":
      return 0;
    case "error":
    case "interrupted":
      return 1;
    case "turn_ended":
      return 2;
    case "working":
      return 3;
    case "session_open":
    case "idle":
      return 4;
    case "closed":
      return 7;
    default:
      return 6;
  }
}

function sortByFocus(sessions: Session[]): Session[] {
  return [...sessions].sort((left, right) => focusRank(left) - focusRank(right) || updatedAt(right) - updatedAt(left));
}

function sortByRecent(sessions: Session[]): Session[] {
  return [...sessions].sort((left, right) => updatedAt(right) - updatedAt(left));
}

function groupedBy(sessions: Session[], keyFor: (session: Session) => string): SessionGroup[] {
  const groups = new Map<string, Session[]>();
  for (const session of sessions) {
    const key = keyFor(session);
    groups.set(key, [...(groups.get(key) ?? []), session]);
  }
  return [...groups.entries()]
    .map(([key, items]) => ({ key, title: key, sessions: sortByFocus(items) }))
    .sort((left, right) => {
      const rank = focusRank(left.sessions[0]) - focusRank(right.sessions[0]);
      if (rank !== 0) return rank;
      return updatedAt(right.sessions[0]) - updatedAt(left.sessions[0]) || left.title.localeCompare(right.title);
    });
}

function organizeSessions(sessions: Session[], organization: Organization): SessionGroup[] {
  if (organization === "recent") {
    return [{ key: "recent", title: "Recent", sessions: sortByRecent(sessions) }];
  }
  if (organization === "project") {
    return groupedBy(sessions, (session) => projectName(session.cwd));
  }
  if (organization === "agent") {
    return groupedBy(sessions, (session) => providerName(session.provider));
  }

  const definitions: Array<{ key: string; title: string; matches: (session: Session) => boolean }> = [
    {
      key: "needs-you",
      title: "Needs You",
      matches: (session) => ["needs_approval", "error", "interrupted", "turn_ended"].includes(session.state.kind),
    },
    {
      key: "active",
      title: "Active",
      matches: (session) => ["working", "session_open", "idle"].includes(session.state.kind),
    },
    {
      key: "recent",
      title: "Recent",
      matches: (session) => session.task?.kind !== "done",
    },
    {
      key: "done",
      title: "Done",
      matches: (session) => session.task?.kind === "done",
    },
  ];
  const remaining = new Set(sessions);
  const groups: SessionGroup[] = [];
  for (const definition of definitions) {
    const items = sortByFocus([...remaining].filter(definition.matches));
    for (const item of items) remaining.delete(item);
    if (items.length > 0) groups.push({ key: definition.key, title: definition.title, sessions: items });
  }
  return groups;
}

function destination(session: Session): string {
  if (session.location?.kind === "tmux") {
    return session.location.tmux_pane ? `tmux ${session.location.tmux_pane}` : "tmux";
  }
  if (session.location?.kind === "wezterm") return "WezTerm";
  const source = session.source?.toLowerCase() ?? "";
  if (source.includes("vscode") || source.includes("vs code")) return "VS Code";
  if (source.includes("chrome")) return "Chrome";
  if (source.includes("safari")) return "Safari";
  if (source.includes("edge")) return "Edge";
  if (source.includes("browser")) return "Browser";
  if (source.includes("desktop")) return "Desktop";
  return "Terminal";
}

function destinationIcon(session: Session) {
  if (session.location?.kind === "tmux") return join(environment.assetsPath, "tmux.svg");
  if (session.location?.kind === "wezterm") return applicationIcon("/Applications/WezTerm.app", Icon.Terminal);
  const source = session.source?.toLowerCase() ?? "";
  if (source.includes("vscode") || source.includes("vs code")) {
    return applicationIcon("/Applications/Visual Studio Code.app", Icon.Code);
  }
  if (source.includes("chrome")) return applicationIcon("/Applications/Google Chrome.app", Icon.Globe01);
  if (source.includes("safari")) return applicationIcon("/Applications/Safari.app", Icon.Globe01);
  if (source.includes("edge")) return applicationIcon("/Applications/Microsoft Edge.app", Icon.Globe01);
  if (source.includes("browser")) return Icon.Globe01;
  if (source.includes("desktop") && session.provider === "codex") {
    return applicationIcon("/Applications/ChatGPT.app", Icon.Desktop);
  }
  if (source.includes("desktop")) return Icon.Desktop;
  return Icon.CommandSymbol;
}

function applicationIcon(path: string, fallback: Icon) {
  return process.platform === "darwin" && existsSync(path) ? { fileIcon: path } : fallback;
}

async function loadSessions(): Promise<Session[]> {
  const { stdout } = await execFileAsync(sesswitch, ["list", "--json"], {
    timeout: 20_000,
    maxBuffer: 8 * 1024 * 1024,
  });
  return JSON.parse(stdout) as Session[];
}

async function openSession(session: Session) {
  const toast = await showToast({
    title: `Opening ${session.title}`,
    style: Toast.Style.Animated,
  });
  try {
    await execFileAsync(sesswitch, ["open", session.key], { timeout: 20_000 });
    await closeMainWindow({ clearRootSearch: true });
  } catch (error) {
    toast.title = "Could not open session";
    toast.message = error instanceof Error ? error.message : String(error);
    toast.style = Toast.Style.Failure;
  }
}

async function markSession(session: Session, mark: "done" | "clear", reload: () => Promise<void>) {
  const title = mark === "done" ? "Marking task done" : "Clearing task mark";
  const toast = await showToast({ title, style: Toast.Style.Animated });
  try {
    await execFileAsync(sesswitch, ["mark", mark, session.key], { timeout: 20_000 });
    await reload();
    toast.title = mark === "done" ? "Task marked done" : "Task mark cleared";
    toast.style = Toast.Style.Success;
  } catch (error) {
    toast.title = "Could not update task mark";
    toast.message = error instanceof Error ? error.message : String(error);
    toast.style = Toast.Style.Failure;
  }
}

function RenameSessionForm({ session, reload }: { session: Session; reload: () => Promise<void> }) {
  const { pop } = useNavigation();

  async function submit(values: Form.Values) {
    const name = String(values.name ?? "").trim();
    if (!name) {
      await showToast({ title: "Name cannot be empty", style: Toast.Style.Failure });
      return false;
    }
    const toast = await showToast({ title: "Renaming session", style: Toast.Style.Animated });
    try {
      await execFileAsync(sesswitch, ["rename", session.key, name], { timeout: 20_000 });
      await reload();
      toast.title = "Session renamed";
      toast.style = Toast.Style.Success;
      pop();
      return true;
    } catch (error) {
      toast.title = "Could not rename session";
      toast.message = error instanceof Error ? error.message : String(error);
      toast.style = Toast.Style.Failure;
      return false;
    }
  }

  return (
    <Form
      navigationTitle={`Rename ${session.title}`}
      actions={
        <ActionPanel>
          <Action.SubmitForm title="Rename Session" icon={Icon.Pencil} onSubmit={submit} />
        </ActionPanel>
      }
    >
      <Form.TextField id="name" title="Name" defaultValue={session.title} autoFocus />
    </Form>
  );
}

export default function AISessions() {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string>();
  const [organization, setOrganization] = useState<Organization>("focus");

  const reload = useCallback(async () => {
    setIsLoading(true);
    setError(undefined);
    try {
      setSessions(await loadSessions());
    } catch (cause) {
      const message = cause instanceof Error ? cause.message : String(cause);
      setError(message);
      await showToast({ title: "Could not load AI sessions", message, style: Toast.Style.Failure });
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    void reload();
  }, [reload]);

  useEffect(() => {
    void LocalStorage.getItem<string>(organizationStorageKey).then((stored) => {
      if (stored === "focus" || stored === "recent" || stored === "project" || stored === "agent") {
        setOrganization(stored);
      }
    });
  }, []);

  const changeOrganization = useCallback((value: Organization) => {
    setOrganization(value);
    void LocalStorage.setItem(organizationStorageKey, value);
  }, []);

  const cycleOrganization = useCallback(() => {
    const order: Organization[] = ["focus", "recent", "project", "agent"];
    const next = order[(order.indexOf(organization) + 1) % order.length];
    changeOrganization(next);
    void showToast({ title: `Organized by ${next[0].toUpperCase()}${next.slice(1)}`, style: Toast.Style.Success });
  }, [changeOrganization, organization]);

  const groups = organizeSessions(sessions, organization);

  return (
    <List
      isLoading={isLoading}
      searchBarPlaceholder="Search sessions, projects, or agents"
      searchBarAccessory={
        <List.Dropdown
          id="organization"
          tooltip="Organize Sessions"
          value={organization}
          onChange={(value) => changeOrganization(value as Organization)}
        >
          <List.Dropdown.Item title="Focus" value="focus" />
          <List.Dropdown.Item title="Recent" value="recent" />
          <List.Dropdown.Item title="Project" value="project" />
          <List.Dropdown.Item title="Agent" value="agent" />
        </List.Dropdown>
      }
    >
      {error ? (
        <List.EmptyView title="Could not load sessions" description={error} icon={Icon.Warning} />
      ) : sessions.length === 0 && !isLoading ? (
        <List.EmptyView title="No AI sessions found" icon={Icon.MagnifyingGlass} />
      ) : (
        groups.map((group) => (
          <List.Section key={group.key} title={`${group.title} (${group.sessions.length})`}>
          {group.sessions.map((session) => {
            const status = statusFor(session);
            const provider = providerName(session.provider);
            const project = projectName(session.cwd);
            const host = destination(session);
            return (
              <List.Item
                key={session.key}
                id={session.key}
                title={session.title}
                subtitle={project}
                icon={{ value: providerIcon(session.provider), tooltip: provider }}
                keywords={[project, provider, host, status.label]}
                accessories={[
                  { icon: destinationIcon(session), tooltip: host },
                  { tag: { value: status.label, color: status.color } },
                ]}
                actions={
                  <ActionPanel>
                    <ActionPanel.Section title="Session">
                      <Action title="Open Session" icon={Icon.ArrowRight} onAction={() => openSession(session)} />
                      {session.provider === "codex" ? (
                        <Action.Push
                          title="Rename Session"
                          icon={Icon.Pencil}
                          shortcut={Keyboard.Shortcut.Common.Edit}
                          target={<RenameSessionForm session={session} reload={reload} />}
                        />
                      ) : null}
                      {session.task?.kind === "done" ? (
                        <Action
                          title="Clear Done Mark"
                          icon={Icon.Circle}
                          shortcut={{ modifiers: ["cmd", "shift"], key: "d" }}
                          onAction={() => markSession(session, "clear", reload)}
                        />
                      ) : (
                        <Action
                          title="Mark Task Done"
                          icon={Icon.CheckCircle}
                          shortcut={{ modifiers: ["cmd", "shift"], key: "d" }}
                          onAction={() => markSession(session, "done", reload)}
                        />
                      )}
                    </ActionPanel.Section>
                    <ActionPanel.Section title="List">
                      <Action
                        title="Cycle Organization"
                        icon={Icon.AppWindowList}
                        shortcut={{ modifiers: ["cmd", "shift"], key: "o" }}
                        onAction={cycleOrganization}
                      />
                      <Action
                        title="Refresh Sessions"
                        icon={Icon.ArrowClockwise}
                        shortcut={Keyboard.Shortcut.Common.Refresh}
                        onAction={reload}
                      />
                      <Action.CopyToClipboard
                        title="Copy Session Key"
                        shortcut={Keyboard.Shortcut.Common.Copy}
                        content={session.key}
                      />
                    </ActionPanel.Section>
                  </ActionPanel>
                }
              />
            );
          })}
          </List.Section>
        ))
      )}
    </List>
  );
}
