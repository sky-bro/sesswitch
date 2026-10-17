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
import { execFile, spawn } from "node:child_process";
import { existsSync } from "node:fs";
import { homedir } from "node:os";
import { join } from "node:path";
import { promisify } from "node:util";
import { useCallback, useEffect, useState } from "react";

import { type Session, type Organization, type StatusFilter, filterSessions, visibleSessionGroups, projectName, displayProjectPath, providerName, statusFilters } from "./session-model";

const execFileAsync = promisify(execFile);
type Preferences = { binaryPath?: string };

function expandHome(path: string): string {
  if (path === "~") return homedir();
  if (path.startsWith("~/")) return join(homedir(), path.slice(2));
  return path;
}

const preferences = getPreferenceValues<Preferences>();
const sesswitch = expandHome(preferences.binaryPath?.trim() || "~/.local/bin/sesswitch");
const statusFilterStorageKey = "session-status-filter";
const organizationStorageKey = "session-organization";
const sessionsCacheStorageKey = `sessions-cache-v1:${sesswitch}`;

function isCachedSession(value: unknown): value is Session {
  if (!value || typeof value !== "object") return false;
  const item = value as Session;
  return typeof item.key === "string" && typeof item.provider === "string" &&
    typeof item.title === "string" && typeof item.updated_at === "string" &&
    !!item.state && typeof item.state.kind === "string" && typeof item.state.source === "string" &&
    (item.cwd === undefined || typeof item.cwd === "string") &&
    (item.source === undefined || typeof item.source === "string") &&
    (item.host === undefined || typeof item.host === "string");
}

type Status = {
  label: string;
  color: Color;
};

function statusFor(session: Session): Status {
  if (session.task?.kind === "done") return { label: "Done", color: Color.Green };
  switch (session.state.kind) {
    case "needs_input":
      return { label: "Needs input", color: Color.Orange };
    case "needs_approval":
      return { label: "Action needed", color: Color.Orange };
    case "working":
      return { label: "Working", color: Color.Blue };
    case "turn_ended":
      return { label: "Ready to review", color: Color.Green };
    case "reviewed":
      return { label: "Reviewed", color: Color.SecondaryText };
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
  if (provider === "codex") return {
    source: {
      light: join(environment.assetsPath, "codex-light.png"),
      dark: join(environment.assetsPath, "codex-dark.png"),
    },
  };
  if (provider === "claude") return join(environment.assetsPath, "claude.svg");
  return Icon.Terminal;
}

function destination(session: Session): string {
  if (session.location?.kind === "tmux") {
    const pane = session.location.tmux_pane ? `tmux ${session.location.tmux_pane}` : "tmux";
    return session.location.display_state === "background" ? `${pane} · another session displayed` : pane;
  }
  if (session.location?.kind === "wezterm") return "WezTerm";
  if (session.host === "tmux") return "tmux";
  if (session.host === "wezterm") return "WezTerm";
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
  if ((session.location?.kind ?? session.host) === "tmux") return join(environment.assetsPath, "tmux.svg");
  if ((session.location?.kind ?? session.host) === "wezterm") return applicationIcon("/Applications/WezTerm.app", Icon.Terminal);
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

async function openSession(session: Session, target?: "host") {
  const toast = await showToast({
    title: `Opening ${session.title}`,
    style: Toast.Style.Animated,
  });
  try {
    await execFileAsync(sesswitch, target ? ["open", "--target", target, session.key] : ["open", session.key], { timeout: 20_000 });
    await closeMainWindow({ clearRootSearch: true });
  } catch (error) {
    toast.title = "Could not open session";
    const stderr = error && typeof error === "object" && "stderr" in error ? String(error.stderr ?? "").trim() : "";
    toast.message = stderr.replace(/^sesswitch:\s*/, "") || (error instanceof Error ? error.message : String(error));
    toast.style = Toast.Style.Failure;
  }
}

async function markSession(session: Session, mark: "read" | "done" | "clear") {
  const title = mark === "read" ? "Marking turn read" : mark === "done" ? "Marking task done" : "Clearing task mark";
  const toast = await showToast({ title, style: Toast.Style.Animated });
  try {
    await execFileAsync(sesswitch, ["mark", mark, session.key], { timeout: 20_000 });
    toast.title = mark === "read" ? "Turn marked read" : mark === "done" ? "Task marked done" : "Task mark cleared";
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
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("all");
  const [organization, setOrganization] = useState<Organization>("focus");
  const [revision, setRevision] = useState(0);
  const [warning, setWarning] = useState<string>();
  const [searchText, setSearchText] = useState("");
  const [expandedProjects, setExpandedProjects] = useState<Set<string>>(new Set());

  // Refresh is explicit: restart the subscription and obtain a fresh catalog.
  const reload = useCallback(async () => {
    setIsLoading(true);
    setRevision((value) => value + 1);
  }, []);

  useEffect(() => {
    let cancelled = false;
    let received = false;
    let failed = false;
    let buffer = "";
    let diagnostics = "";
    const child = spawn(sesswitch, ["watch"], { stdio: ["ignore", "pipe", "pipe"] });
    const fail = (message: string) => {
      if (cancelled || failed) return;
      failed = true;
      setIsLoading(false);
      setError(message);
      void showToast({ title: "Session updates stopped", message, style: Toast.Style.Failure });
    };
    setError(undefined);
    void LocalStorage.getItem<string>(sessionsCacheStorageKey).then((cached) => {
      if (!cached || cancelled || received) return;
      const parsed = JSON.parse(cached) as Session[];
      if (Array.isArray(parsed) && parsed.every(isCachedSession)) setSessions(parsed);
    }).catch(() => undefined);
    child.stdout.setEncoding("utf8");
    child.stdout.on("data", (chunk: string) => {
      if (cancelled) return;
      buffer += chunk;
      let newline: number;
      while ((newline = buffer.indexOf("\n")) >= 0) {
        const line = buffer.slice(0, newline);
        buffer = buffer.slice(newline + 1);
        try {
          const snapshot = JSON.parse(line) as { sessions: Session[]; warnings?: string[] };
          if (!Array.isArray(snapshot.sessions) || !snapshot.sessions.every(isCachedSession)) {
            throw new Error("Invalid session snapshot");
          }
          received = true;
          setSessions(snapshot.sessions);
          setWarning(snapshot.warnings?.join("; "));
          setError(undefined);
          setIsLoading(false);
          void LocalStorage.setItem(sessionsCacheStorageKey, JSON.stringify(snapshot.sessions)).catch(() => undefined);
        } catch (cause) {
          fail(cause instanceof Error ? cause.message : String(cause));
          child.kill();
          return;
        }
      }
    });
    child.stderr.setEncoding("utf8");
    child.stderr.on("data", (chunk: string) => { diagnostics = (diagnostics + chunk).slice(-4096); });
    child.on("error", (cause) => fail(cause.message));
    child.on("close", () => fail(diagnostics.trim() || "Refresh Sessions to reconnect."));
    return () => {
      cancelled = true;
      child.kill();
    };
  }, [revision]);

  useEffect(() => {
    void LocalStorage.getItem<string>(statusFilterStorageKey).then((stored) => {
      if (statusFilters.some((filter) => filter.key === stored)) setStatusFilter(stored as StatusFilter);
    }).catch(() => undefined);
    void LocalStorage.getItem<string>(organizationStorageKey).then((stored) => {
      if (stored === "focus" || stored === "recent" || stored === "project" || stored === "agent") {
        setOrganization(stored);
      }
    }).catch(() => undefined);
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

  const changeStatusFilter = (value: StatusFilter) => {
    setStatusFilter(value);
    void LocalStorage.setItem(statusFilterStorageKey, value);
  };
  const filtered = filterSessions(sessions, statusFilter);
  const visibleCount = filterSessions(sessions, "all").length;
  const groups = visibleSessionGroups(sessions, organization, statusFilter, expandedProjects, searchText);
  const toggleProjectHistory = (key: string) => setExpandedProjects((previous) => {
    const next = new Set(previous);
    if (next.has(key)) next.delete(key);
    else next.add(key);
    return next;
  });
  const listActions = (
    <ActionPanel.Section title="List">
      <ActionPanel.Submenu title="Organize By" icon={Icon.AppWindowList} shortcut={{ modifiers: ["cmd", "shift"], key: "g" }}>
        {(["focus", "recent", "project", "agent"] as Organization[]).map((value) => (
          <Action key={value} title={value[0].toUpperCase() + value.slice(1)} onAction={() => changeOrganization(value)} />
        ))}
      </ActionPanel.Submenu>
      <ActionPanel.Submenu title="Filter Status" icon={Icon.MagnifyingGlass}>
        {statusFilters.map((filter) => (
          <Action key={filter.key} title={`${filter.title} (${filterSessions(sessions, filter.key).length})`} onAction={() => changeStatusFilter(filter.key)} />
        ))}
      </ActionPanel.Submenu>
      <Action title="Cycle Organization" icon={Icon.AppWindowList} shortcut={{ modifiers: ["cmd", "shift"], key: "o" }} onAction={cycleOrganization} />
      <Action title="Refresh Sessions" icon={Icon.ArrowClockwise} shortcut={Keyboard.Shortcut.Common.Refresh} onAction={() => reload()} />
    </ActionPanel.Section>
  );

  return (
    <List
      isLoading={isLoading}
      navigationTitle={`AI Sessions · ${organization[0].toUpperCase()}${organization.slice(1)}${statusFilter === "all" ? "" : ` · ${statusFilters.find((filter) => filter.key === statusFilter)?.title}`}`}
      actions={<ActionPanel>{listActions}</ActionPanel>}
      searchBarPlaceholder="Search sessions, projects, or agents"
      searchText={searchText}
      onSearchTextChange={setSearchText}
      filtering={true}
      searchBarAccessory={
        <List.Dropdown
          id="session-organization"
          tooltip="Organize By"
          value={organization}
          onChange={(value) => changeOrganization(value as Organization)}
        >
          {(["focus", "recent", "project", "agent"] as Organization[]).map((value) => (
            <List.Dropdown.Item key={value} title={value[0].toUpperCase() + value.slice(1)} value={value} />
          ))}
        </List.Dropdown>
      }
    >
      {(warning || error) && (sessions.length > 0 || !!warning) ? (
        <List.Section title={error ? "Updates stopped — refresh to reconnect" : "Some providers are unavailable"}>
          <List.Item id="session-warning" title={error || warning || ""} icon={Icon.Warning} actions={<ActionPanel><Action title="Refresh Sessions" icon={Icon.ArrowClockwise} onAction={() => reload()} /></ActionPanel>} />
        </List.Section>
      ) : null}
      {error && sessions.length === 0 ? (
        <List.EmptyView title="Could not load sessions" description={error} icon={Icon.Warning} />
      ) : filtered.length === 0 && !isLoading ? (
        <List.EmptyView title={visibleCount ? "No sessions match this status" : "No AI sessions found"} description={visibleCount ? "Choose Filter Status → All in the actions menu to see every session." : undefined} icon={Icon.MagnifyingGlass} />
      ) : (
        groups.map((group) => (
          <List.Section key={group.key} title={`${group.title} (${group.totalCount})`}>
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
                keywords={[project, displayProjectPath(session.cwd ?? ""), provider, host, status.label]}
                accessories={[
                  { icon: destinationIcon(session), tooltip: host },
                  { tag: { value: status.label, color: status.color } },
                  ...(session.resume === "unavailable" ? [{ text: "No saved data", tooltip: "May be a temporary /side conversation. It cannot be independently resumed; use the original CLI to switch back if it is still available." }] : session.location?.display_state === "background" ? [{ text: "Background", tooltip: "This CLI is currently displaying another session." }] : []),
                ]}
                actions={
                  <ActionPanel>
                    <ActionPanel.Section title="Session">
                      {session.resume === "unavailable" ? (
                        session.location ? <Action title="Go to Original CLI" icon={Icon.Terminal} onAction={() => openSession(session, "host")} /> : <Action title="Session Cannot Be Resumed" icon={Icon.Warning} onAction={() => showToast({ title: "No saved session data", message: "The original CLI is unavailable. This may have been a temporary /side conversation.", style: Toast.Style.Failure })} />
                      ) : <Action title="Open Session" icon={Icon.ArrowRight} onAction={() => openSession(session)} />}
                      {session.provider === "codex" && session.resume !== "unavailable" ? (
                        <Action.Push
                          title="Rename Session"
                          icon={Icon.Pencil}
                          shortcut={Keyboard.Shortcut.Common.Edit}
                          target={<RenameSessionForm session={session} reload={reload} />}
                        />
                      ) : null}
                      {session.state.kind === "turn_ended" ? (
                        <Action
                          title="Mark Turn Read"
                          icon={Icon.Checkmark}
                          shortcut={{ modifiers: ["cmd", "shift"], key: "r" }}
                          onAction={() => markSession(session, "read")}
                        />
                      ) : null}
                      {session.task?.kind === "done" ? (
                        <Action
                          title="Clear Done Mark"
                          icon={Icon.Circle}
                          shortcut={{ modifiers: ["cmd", "shift"], key: "d" }}
                          onAction={() => markSession(session, "clear")}
                        />
                      ) : (
                        <Action
                          title="Mark Task Done"
                          icon={Icon.CheckCircle}
                          shortcut={{ modifiers: ["cmd", "shift"], key: "d" }}
                          onAction={() => markSession(session, "done")}
                        />
                      )}
                    </ActionPanel.Section>
                    {group.historyCount > 0 ? (
                      <Action title={group.historyExpanded ? "Hide Project History" : "Show Project History"} icon={group.historyExpanded ? Icon.ChevronUp : Icon.ChevronDown} onAction={() => toggleProjectHistory(group.key)} />
                    ) : null}
                    {listActions}
                    <ActionPanel.Section>
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
          {group.historyCount > 0 ? (
            <List.Item
              id={`project-history:${group.key}`}
              title={`${group.historyExpanded ? "Hide" : "Show"} ${group.historyCount} past sessions`}
              subtitle="Closed, saved, reviewed, or done"
              icon={group.historyExpanded ? Icon.ChevronUp : Icon.ChevronDown}
              keywords={[group.title]}
              actions={
                <ActionPanel>
                  <Action title={group.historyExpanded ? "Hide Past Sessions" : "Show Past Sessions"} icon={group.historyExpanded ? Icon.ChevronUp : Icon.ChevronDown} onAction={() => toggleProjectHistory(group.key)} />
                  {listActions}
                </ActionPanel>
              }
            />
          ) : null}
          </List.Section>
        ))
      )}
    </List>
  );
}
