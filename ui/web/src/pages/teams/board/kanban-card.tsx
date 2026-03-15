import { memo } from "react";
import { Badge } from "@/components/ui/badge";
import { useTranslation } from "react-i18next";
import { isTaskLocked } from "./board-utils";
import type { TeamTaskData } from "@/types/team";

const PRIORITY_COLORS: Record<number, string> = {
  0: "bg-slate-400",
  1: "bg-blue-500",
  2: "bg-amber-500",
  3: "bg-red-500",
};

interface KanbanCardProps {
  task: TeamTaskData;
  isTeamV2?: boolean;
  onClick: () => void;
}

export const KanbanCard = memo(function KanbanCard({ task, isTeamV2, onClick }: KanbanCardProps) {
  const { t } = useTranslation("teams");
  const locked = isTaskLocked(task);
  const blocked = task.status === "blocked";

  return (
    <div
      className={
        "cursor-pointer rounded-lg border bg-card p-3 shadow-sm transition-colors hover:bg-accent/50" +
        (locked ? " border-l-2 border-l-green-500" : blocked ? " border-l-2 border-l-amber-500" : "")
      }
      onClick={onClick}
    >
      <div className="mb-1 flex items-center justify-between">
        <span className="font-mono text-[10px] text-muted-foreground">
          {task.identifier || `#${task.task_number ?? ""}`}
        </span>
        <div className="flex items-center gap-1.5">
          {locked && (
            <span className="flex items-center gap-1 text-[10px] text-green-600 dark:text-green-400">
              <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-green-500" />
              {t("board.running")}
            </span>
          )}
          <span
            className={`h-2 w-2 rounded-full ${PRIORITY_COLORS[task.priority] ?? PRIORITY_COLORS[0]}`}
            title={`Priority ${task.priority}`}
          />
        </div>
      </div>

      <p className="line-clamp-2 text-sm font-medium leading-snug">{task.subject}</p>

      <div className="mt-2 flex items-center gap-1.5">
        <span className="truncate text-xs text-muted-foreground">
          {task.owner_agent_key || t("board.unassigned")}
        </span>
        {task.task_type && task.task_type !== "general" && (
          <Badge variant="outline" className="text-[10px] px-1 py-0">{task.task_type}</Badge>
        )}
      </div>

      {isTeamV2 && task.progress_percent != null && task.progress_percent > 0 && (
        <div className="mt-2 flex items-center gap-1.5">
          <div className="h-1.5 flex-1 rounded-full bg-muted">
            <div className="h-full rounded-full bg-primary transition-all" style={{ width: `${task.progress_percent}%` }} />
          </div>
          <span className="text-[10px] text-muted-foreground">{task.progress_percent}%</span>
        </div>
      )}
    </div>
  );
});
