import { normalize, sep } from "node:path";
import { homedir } from "node:os";

export type State = {
  kind: string;
  source: string;
  observed_at?: string;
};

export type Location = {
  kind: string;
  tmux_pane?: string;
  needs_attention?: boolean;
  display_state?: "current" | "background" | "unknown";
};

export type Session = {
  key: string;
  provider: string;
  title: string;
  cwd?: string;
  source?: string;
  host?: string;
  resume?: "unavailable";
  updated_at: string;
  state: State;
  location?: Location;
  task?: { kind: string };
};

export type Organization = "focus" | "recent" | "project" | "agent";

export type SessionGroup = {
  key: string;
  title: string;
  sessions: Session[];
};


export type StatusFilter = "all" | "needs-you" | "working" | "ready" | "inactive" | "done";
export const statusFilters: Array<{ key: StatusFilter; title: string }> = [
  { key: "all", title: "All" }, { key: "needs-you", title: "Needs You" },
  { key: "working", title: "Working" }, { key: "ready", title: "Ready" },
  { key: "inactive", title: "Inactive" }, { key: "done", title: "Done" },
];
export function statusCategory(session: Session): Exclude<StatusFilter, "all"> {
  if (session.task?.kind === "done") return "done";
  if (["needs_approval", "needs_input", "error", "interrupted"].includes(session.state.kind)) return "needs-you";
  if (session.state.kind === "working") return "working";
  if (["turn_ended", "idle", "session_open"].includes(session.state.kind)) return "ready";
  return "inactive";
}
export function filterSessions(sessions: Session[], filter: StatusFilter): Session[] {
  // Cached snapshots from an older CLI follow the same visibility rules.
  return sessions.filter((session) => {
    if (session.resume === "unavailable" && (!session.location || session.task?.kind === "done" || !["needs_approval", "needs_input"].includes(session.state.kind))) return false;
    return filter === "all" || statusCategory(session) === filter;
  });
}

export function visibleSessionGroups(
  sessions: Session[], organization: Organization, filter: StatusFilter,
  expandedProjects: ReadonlySet<string>, searchText: string,
): Array<SessionGroup & { totalCount: number; historyCount: number; historyExpanded: boolean }> {
  const foldHistory = organization === "project" && filter === "all" && !searchText.trim();
  return organizeSessions(filterSessions(sessions, filter), organization).map((group) => {
    const history = (session: Session) => ["inactive", "done"].includes(statusCategory(session));
    const historyCount = foldHistory ? group.sessions.filter(history).length : 0;
    const historyExpanded = expandedProjects.has(group.key);
    return {
      ...group,
      totalCount: group.sessions.length,
      historyCount,
      historyExpanded,
      sessions: foldHistory && !historyExpanded ? group.sessions.filter((session) => !history(session)) : group.sessions,
    };
  });
}
export function projectKey(cwd?: string): string {
  if (!cwd?.trim()) return "";
  let path = normalize(cwd);
  while (path.length > 1 && path.endsWith(sep)) path = path.slice(0, -1);
  return path;
}
export function displayProjectPath(path: string): string {
  if (!path) return "Unknown project";
  const home = homedir();
  if (path === home) return "~";
  return path.startsWith(home + sep) ? "~" + path.slice(home.length) : path;
}
export function providerName(provider: string): string {
  if (provider === "codex") return "Codex";
  if (provider === "claude") return "Claude";
  return provider;
}

export function projectName(cwd?: string): string {
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
    case "needs_input":
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

function groupedBy(sessions: Session[], keyFor: (session: Session) => string, titleFor: (key: string) => string = (key) => key): SessionGroup[] {
  const groups = new Map<string, Session[]>();
  for (const session of sessions) {
    const key = keyFor(session);
    groups.set(key, [...(groups.get(key) ?? []), session]);
  }
  return [...groups.entries()]
    .map(([key, items]) => ({ key, title: titleFor(key), sessions: sortByFocus(items) }))
    .sort((left, right) => {
      const rank = focusRank(left.sessions[0]) - focusRank(right.sessions[0]);
      if (rank !== 0) return rank;
      return updatedAt(right.sessions[0]) - updatedAt(left.sessions[0]) || left.title.localeCompare(right.title);
    });
}

export function organizeSessions(sessions: Session[], organization: Organization): SessionGroup[] {
  if (organization === "recent") {
    return [{ key: "recent", title: "Recent", sessions: sortByRecent(sessions) }];
  }
  if (organization === "project") {
    return groupedBy(sessions, (session) => projectKey(session.cwd), displayProjectPath);
  }
  if (organization === "agent") {
    return groupedBy(sessions, (session) => providerName(session.provider));
  }

  const definitions: Array<{ key: string; title: string; matches: (session: Session) => boolean }> = [
    {
      key: "needs-you",
      title: "Needs You",
      matches: (session) => session.task?.kind !== "done" && ["needs_approval", "needs_input", "error", "interrupted", "turn_ended"].includes(session.state.kind),
    },
    {
      key: "active",
      title: "Active",
      matches: (session) => session.task?.kind !== "done" && ["working", "session_open", "idle"].includes(session.state.kind),
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
