import { useState } from "react";
import { KeyRound, Plus, RefreshCw, Pencil, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { PageHeader } from "@/components/shared/page-header";
import { EmptyState } from "@/components/shared/empty-state";
import { TableSkeleton } from "@/components/shared/loading-skeleton";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { useMinLoading } from "@/hooks/use-min-loading";
import { useDeferredLoading } from "@/hooks/use-deferred-loading";
import { useCliCredentials, useCliCredentialPresets } from "./hooks/use-cli-credentials";
import { CliCredentialFormDialog } from "./cli-credential-form-dialog";
import type { SecureCLIBinary, CLICredentialInput } from "./hooks/use-cli-credentials";

export function CliCredentialsPage() {
  const [formOpen, setFormOpen] = useState(false);
  const [editItem, setEditItem] = useState<SecureCLIBinary | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<SecureCLIBinary | null>(null);
  const [deleteLoading, setDeleteLoading] = useState(false);

  const { items, loading, refresh, createCredential, updateCredential, deleteCredential } =
    useCliCredentials();
  const { presets } = useCliCredentialPresets();

  const spinning = useMinLoading(loading);
  const showSkeleton = useDeferredLoading(loading && items.length === 0);

  const handleCreate = async (data: CLICredentialInput) => {
    await createCredential(data);
  };

  const handleEdit = async (data: CLICredentialInput) => {
    if (!editItem) return;
    await updateCredential(editItem.id, data);
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleteLoading(true);
    try {
      await deleteCredential(deleteTarget.id);
      setDeleteTarget(null);
    } finally {
      setDeleteLoading(false);
    }
  };

  const openCreate = () => {
    setEditItem(null);
    setFormOpen(true);
  };

  const openEdit = (item: SecureCLIBinary) => {
    setEditItem(item);
    setFormOpen(true);
  };

  return (
    <div className="p-4 sm:p-6">
      <PageHeader
        title="CLI Credentials"
        description="Manage secure CLI binaries available to agents with injected credentials."
        actions={
          <div className="flex gap-2">
            <Button size="sm" onClick={openCreate} className="gap-1">
              <Plus className="h-3.5 w-3.5" /> Add Credential
            </Button>
            <Button variant="outline" size="sm" onClick={refresh} disabled={spinning} className="gap-1">
              <RefreshCw className={"h-3.5 w-3.5" + (spinning ? " animate-spin" : "")} /> Refresh
            </Button>
          </div>
        }
      />

      <div className="mt-4">
        {showSkeleton ? (
          <TableSkeleton rows={5} />
        ) : items.length === 0 ? (
          <EmptyState
            icon={KeyRound}
            title="No CLI credentials yet"
            description="Add a CLI credential to allow agents to run authenticated CLI commands."
          />
        ) : (
          <div className="overflow-x-auto rounded-md border">
            <table className="w-full min-w-[600px] text-sm">
              <thead>
                <tr className="border-b bg-muted/50">
                  <th className="px-4 py-3 text-left font-medium">Binary</th>
                  <th className="px-4 py-3 text-left font-medium">Description</th>
                  <th className="px-4 py-3 text-left font-medium">Scope</th>
                  <th className="px-4 py-3 text-left font-medium">Enabled</th>
                  <th className="px-4 py-3 text-left font-medium">Timeout</th>
                  <th className="px-4 py-3 text-right font-medium">Actions</th>
                </tr>
              </thead>
              <tbody>
                {items.map((item) => (
                  <tr key={item.id} className="border-b last:border-0 hover:bg-muted/30">
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <KeyRound className="h-4 w-4 shrink-0 text-muted-foreground" />
                        <div>
                          <div className="font-medium">{item.binary_name}</div>
                          {item.binary_path && (
                            <div className="text-xs text-muted-foreground font-mono">{item.binary_path}</div>
                          )}
                        </div>
                      </div>
                    </td>
                    <td className="px-4 py-3 text-muted-foreground max-w-[220px] truncate">
                      {item.description || "—"}
                    </td>
                    <td className="px-4 py-3">
                      <Badge variant={item.agent_id ? "secondary" : "outline"}>
                        {item.agent_id ? "Agent" : "Global"}
                      </Badge>
                    </td>
                    <td className="px-4 py-3">
                      <Badge variant={item.enabled ? "default" : "secondary"}>
                        {item.enabled ? "Enabled" : "Disabled"}
                      </Badge>
                    </td>
                    <td className="px-4 py-3 text-muted-foreground">{item.timeout_seconds}s</td>
                    <td className="px-4 py-3 text-right">
                      <div className="flex items-center justify-end gap-1">
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => openEdit(item)}
                          className="gap-1"
                        >
                          <Pencil className="h-3.5 w-3.5" /> Edit
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => setDeleteTarget(item)}
                          className="gap-1 text-destructive hover:text-destructive"
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <CliCredentialFormDialog
        open={formOpen}
        onOpenChange={setFormOpen}
        credential={editItem}
        presets={presets}
        onSubmit={editItem ? handleEdit : handleCreate}
      />

      <ConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title="Delete CLI Credential"
        description={`Remove "${deleteTarget?.binary_name}"? Agents that depend on it will lose access.`}
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={handleDelete}
        loading={deleteLoading}
      />
    </div>
  );
}
