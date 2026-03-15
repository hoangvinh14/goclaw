import { useState, useEffect } from "react";
import {
  Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from "@/components/ui/select";
import type { SecureCLIBinary, CLICredentialInput, CLIPreset } from "./hooks/use-cli-credentials";

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  credential?: SecureCLIBinary | null;
  presets: Record<string, CLIPreset>;
  onSubmit: (data: CLICredentialInput) => Promise<unknown>;
}

const NONE_PRESET = "__none__";

export function CliCredentialFormDialog({ open, onOpenChange, credential, presets, onSubmit }: Props) {
  const [selectedPreset, setSelectedPreset] = useState(NONE_PRESET);
  const [binaryName, setBinaryName] = useState("");
  const [binaryPath, setBinaryPath] = useState("");
  const [description, setDescription] = useState("");
  const [denyArgs, setDenyArgs] = useState("");
  const [denyVerbose, setDenyVerbose] = useState("");
  const [timeout, setTimeout] = useState(30);
  const [tips, setTips] = useState("");
  const [agentId, setAgentId] = useState("");
  const [enabled, setEnabled] = useState(true);
  const [envValues, setEnvValues] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const isEdit = !!credential;
  // Build typed entry list to avoid noUncheckedIndexedAccess issues
  const presetEntries: Array<[string, CLIPreset]> = Object.entries(presets).filter(
    (e): e is [string, CLIPreset] => e[1] !== undefined,
  );

  // Current preset definition (for env var fields)
  const activePreset: CLIPreset | null =
    selectedPreset !== NONE_PRESET ? (presets[selectedPreset] ?? null) : null;

  useEffect(() => {
    if (!open) return;
    setSelectedPreset(NONE_PRESET);
    setBinaryName(credential?.binary_name ?? "");
    setBinaryPath(credential?.binary_path ?? "");
    setDescription(credential?.description ?? "");
    setDenyArgs((credential?.deny_args ?? []).join(", "));
    setDenyVerbose((credential?.deny_verbose ?? []).join(", "));
    setTimeout(credential?.timeout_seconds ?? 30);
    setTips(credential?.tips ?? "");
    setAgentId(credential?.agent_id ?? "");
    setEnabled(credential?.enabled ?? true);
    setEnvValues({});
    setError("");
  }, [open, credential]);

  const applyPreset = (key: string) => {
    setSelectedPreset(key);
    if (key === NONE_PRESET) return;
    const p = presets[key];
    if (!p) return;
    setBinaryName(p.binary_name);
    setDescription(p.description);
    setDenyArgs(p.deny_args.join(", "));
    setDenyVerbose(p.deny_verbose.join(", "));
    setTimeout(p.timeout);
    setTips(p.tips);
    setEnvValues({});
  };

  const splitCommaList = (v: string): string[] =>
    v.split(",").map((s) => s.trim()).filter(Boolean);

  const handleSubmit = async () => {
    if (!binaryName.trim()) {
      setError("Binary name is required.");
      return;
    }
    setLoading(true);
    setError("");
    try {
      const payload: CLICredentialInput = {
        binary_name: binaryName.trim(),
        binary_path: binaryPath.trim() || undefined,
        description: description.trim(),
        deny_args: splitCommaList(denyArgs),
        deny_verbose: splitCommaList(denyVerbose),
        timeout_seconds: timeout,
        tips: tips.trim(),
        agent_id: agentId.trim() || undefined,
        enabled,
      };
      if (selectedPreset !== NONE_PRESET) payload.preset = selectedPreset;
      if (Object.keys(envValues).length > 0) payload.env = envValues;
      await onSubmit(payload);
      onOpenChange(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save.");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(v) => !loading && onOpenChange(v)}>
      <DialogContent className="max-h-[85vh] flex flex-col sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{isEdit ? "Edit CLI Credential" : "Add CLI Credential"}</DialogTitle>
        </DialogHeader>

        <div className="grid gap-4 py-2 px-0.5 -mx-0.5 overflow-y-auto min-h-0">
          {/* Preset selector — only on create */}
          {!isEdit && presetEntries.length > 0 && (
            <div className="grid gap-1.5">
              <Label>Preset</Label>
              <Select value={selectedPreset} onValueChange={applyPreset}>
                <SelectTrigger className="text-base md:text-sm">
                  <SelectValue placeholder="Select a preset (optional)" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={NONE_PRESET}>No preset (manual)</SelectItem>
                  {presetEntries.map(([k, p]) => (
                    <SelectItem key={k} value={k}>
                      {p.binary_name} — {p.description}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <p className="text-xs text-muted-foreground">
                Selecting a preset auto-fills fields and shows required env vars.
              </p>
            </div>
          )}


          {/* Existing credentials indicator (edit mode) */}
          {isEdit && (
            <p className="text-xs text-muted-foreground rounded-md border border-dashed p-2">
              Credentials are encrypted and cannot be displayed. Leave environment variables empty to keep existing values, or re-enter to update.
            </p>
          )}
          {/* Env var inputs from preset */}
          {activePreset && activePreset.env_vars.length > 0 && (
            <div className="grid gap-3 rounded-md border p-3">
              <p className="text-sm font-medium">Environment Variables</p>
              {activePreset.env_vars.map((ev) => (
                <div key={ev.name} className="grid gap-1.5">
                  <Label htmlFor={`env-${ev.name}`}>
                    {ev.name}
                    {ev.optional && <span className="ml-1 text-xs text-muted-foreground">(optional)</span>}
                  </Label>
                  <Input
                    id={`env-${ev.name}`}
                    type="password"
                    autoComplete="off"
                    placeholder={ev.desc}
                    value={envValues[ev.name] ?? ""}
                    onChange={(e) => setEnvValues((prev) => ({ ...prev, [ev.name]: e.target.value }))}
                    className="text-base md:text-sm"
                  />
                  {ev.desc && (
                    <p className="text-xs text-muted-foreground">{ev.desc}</p>
                  )}
                </div>
              ))}
            </div>
          )}

          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div className="grid gap-1.5">
              <Label htmlFor="cc-name">Binary Name</Label>
              <Input
                id="cc-name"
                value={binaryName}
                onChange={(e) => setBinaryName(e.target.value)}
                placeholder="e.g. gh"
                className="text-base md:text-sm"
              />
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="cc-path">Binary Path <span className="text-xs text-muted-foreground">(optional)</span></Label>
              <Input
                id="cc-path"
                value={binaryPath}
                onChange={(e) => setBinaryPath(e.target.value)}
                placeholder="/usr/local/bin/gh"
                className="text-base md:text-sm"
              />
            </div>
          </div>

          <div className="grid gap-1.5">
            <Label htmlFor="cc-desc">Description</Label>
            <Textarea
              id="cc-desc"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="What this CLI tool does"
              rows={2}
              className="text-base md:text-sm"
            />
          </div>

          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div className="grid gap-1.5">
              <Label htmlFor="cc-deny-args">Deny Args <span className="text-xs text-muted-foreground">(comma-separated)</span></Label>
              <Input
                id="cc-deny-args"
                value={denyArgs}
                onChange={(e) => setDenyArgs(e.target.value)}
                placeholder="--admin, --force"
                className="text-base md:text-sm"
              />
            </div>
            <div className="grid gap-1.5">
              <Label htmlFor="cc-timeout">Timeout (s)</Label>
              <Input
                id="cc-timeout"
                type="number"
                min={1}
                value={timeout}
                onChange={(e) => setTimeout(Number(e.target.value))}
                className="text-base md:text-sm"
              />
            </div>
          </div>

          <div className="grid gap-1.5">
            <Label htmlFor="cc-deny-verbose">Deny Verbose Args <span className="text-xs text-muted-foreground">(comma-separated)</span></Label>
            <Input
              id="cc-deny-verbose"
              value={denyVerbose}
              onChange={(e) => setDenyVerbose(e.target.value)}
              placeholder="-v, --verbose"
              className="text-base md:text-sm"
            />
          </div>

          <div className="grid gap-1.5">
            <Label htmlFor="cc-tips">Tips</Label>
            <Textarea
              id="cc-tips"
              value={tips}
              onChange={(e) => setTips(e.target.value)}
              placeholder="Usage tips for the agent"
              rows={2}
              className="text-base md:text-sm"
            />
          </div>

          <div className="grid gap-1.5">
            <Label htmlFor="cc-agent">Agent ID <span className="text-xs text-muted-foreground">(optional — leave blank for global)</span></Label>
            <Input
              id="cc-agent"
              value={agentId}
              onChange={(e) => setAgentId(e.target.value)}
              placeholder="agent-id"
              className="text-base md:text-sm"
            />
          </div>

          <div className="flex items-center gap-2">
            <Switch id="cc-enabled" checked={enabled} onCheckedChange={setEnabled} />
            <Label htmlFor="cc-enabled">Enabled</Label>
          </div>

          {error && <p className="text-sm text-destructive">{error}</p>}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={loading}>Cancel</Button>
          <Button onClick={handleSubmit} disabled={loading}>
            {loading ? "Saving..." : isEdit ? "Update" : "Create"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
