import assert from "node:assert/strict";
import { test } from "node:test";
import { homedir } from "node:os";
import { filterSessions, organizeSessions, visibleSessionGroups, displayProjectPath, type Session } from "../src/session-model.ts";

const item = (key: string, cwd: string, kind: string, done = false): Session => ({
  key, cwd, title: key, provider: "codex", updated_at: "2026-01-01T00:00:00Z",
  state: { kind, source: "test" }, task: done ? { kind: "done" } : undefined,
});

test("non-resumable history stays out of filters, search groups, and counts", () => {
  const temporary = { ...item("side", "/tmp/a", "turn_ended"), resume: "unavailable" as const, location: { kind: "tmux" } };
  const ordinary = item("saved", "/tmp/b", "closed");
  assert.deepEqual(filterSessions([temporary, ordinary], "all").map((s) => s.key), ["saved"]);
  assert.equal(visibleSessionGroups([temporary, ordinary], "project", "all", new Set(), "side").length, 1);
  const request = { ...temporary, state: { kind: "needs_input", source: "hook" } };
  assert.deepEqual(filterSessions([request], "needs-you").map((s) => s.key), ["side"]);
  assert.equal(filterSessions([{ ...request, location: undefined }], "all").length, 0);
  assert.equal(filterSessions([{ ...request, task: { kind: "done" } }], "all").length, 0);
  assert.equal(filterSessions([{ ...temporary, resume: undefined }], "all").length, 1);
});

test("project history folds without hiding active sessions or other projects", () => {
  const sessions = [
    item("work", "/tmp/a", "working"), item("ready", "/tmp/a", "turn_ended"),
    item("input", "/tmp/a", "needs_input"),
    ...Array.from({ length: 100 }, (_, i) => item(`closed-${i}`, "/tmp/a", "closed")),
    item("saved", "/tmp/a", "saved"), item("reviewed", "/tmp/a", "reviewed"),
    item("done", "/tmp/a", "idle", true), item("other", "/tmp/b", "closed"),
  ];
  const groups = visibleSessionGroups(sessions, "project", "all", new Set(), "");
  const a = groups.find((g) => g.key === "/tmp/a")!;
  assert.deepEqual(a.sessions.map((s) => s.key), ["input", "ready", "work"]);
  assert.equal(a.historyCount, 103);
  assert.equal(a.totalCount, 106);
  const b = groups.find((g) => g.key === "/tmp/b")!;
  assert.equal(b.sessions.length, 0);
  assert.equal(b.historyCount, 1);

  const expanded = visibleSessionGroups(sessions, "project", "all", new Set(["/tmp/a"]), "");
  assert.equal(expanded.find((g) => g.key === "/tmp/a")?.sessions.length, 106);
  assert.equal(expanded.find((g) => g.key === "/tmp/b")?.sessions.length, 0);
  assert.equal(visibleSessionGroups(sessions, "project", "all", new Set(), "closed").flatMap((g) => g.sessions).length, sessions.length);
  assert.equal(visibleSessionGroups(sessions, "project", "inactive", new Set(), "").flatMap((g) => g.sessions).length, 103);
  assert.equal(visibleSessionGroups(sessions, "project", "done", new Set(), "").flatMap((g) => g.sessions).length, 1);
  assert.equal(visibleSessionGroups(sessions, "recent", "all", new Set(), "")[0].sessions.length, sessions.length);
});

test("project identity separates same-name directories and normalizes paths", () => {
  const groups = organizeSessions([
    item("a", "/tmp/team-a/home-lab", "working"),
    item("b", "/tmp/team-b/home-lab", "needs_input"),
    item("c", "/tmp/team-a/home-lab/", "idle"),
  ], "project");
  assert.equal(groups.length, 2);
  assert.equal(groups.find((g) => g.key === "/tmp/team-a/home-lab")?.sessions.length, 2);
  assert.equal(groups[0].key, "/tmp/team-b/home-lab");
  assert.equal(displayProjectPath(homedir() + "/repos/home-lab"), "~/repos/home-lab");
  assert.equal(displayProjectPath(homedir() + "-other/repo"), homedir() + "-other/repo");
});

test("status filtering remains independent of project grouping and explicit Done", () => {
  const sessions = [
    item("approval", "/tmp/a", "needs_approval"), item("input", "/tmp/b", "needs_input"),
    item("work", "/tmp/a", "working"), item("turn", "/tmp/a", "turn_ended"),
    item("closed", "/tmp/a", "closed"), item("done", "/tmp/b", "needs_input", true),
  ];
  const needs = filterSessions(sessions, "needs-you");
  assert.deepEqual(needs.map((s) => s.key), ["approval", "input"]);
  assert.equal(organizeSessions(needs, "project").length, 2);
  assert.deepEqual(filterSessions(sessions, "ready").map((s) => s.key), ["turn"]);
  assert.deepEqual(filterSessions(sessions, "inactive").map((s) => s.key), ["closed"]);
  assert.deepEqual(filterSessions(sessions, "done").map((s) => s.key), ["done"]);
  assert.equal(filterSessions(sessions, "all").length, sessions.length);
  assert.equal(filterSessions(sessions, "working")[0].key, "work");
});
