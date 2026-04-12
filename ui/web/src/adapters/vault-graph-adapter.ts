import Graph from "graphology";
import type { VaultDocument, VaultLink } from "@/types/vault";
import { getNodeSize, truncateMiddle } from "@/components/graph/graph-utils";

// Colors per vault document type — muted pastels that work well on both
// light and dark backgrounds. Avoid harsh neons on dark mode.
export const VAULT_TYPE_COLORS: Record<string, string> = {
  context: "#818cf8",  // indigo-400 (softer)
  memory: "#a78bfa",   // violet-400
  note: "#5eead4",     // teal-300 (soft, readable)
  skill: "#86efac",    // green-300
  episodic: "#fcd34d", // amber-300
  media: "#f9a8d4",    // pink-300 (soft, not harsh)
  document: "#67e8f9", // cyan-300
};
const DEFAULT_COLOR = "#a1a1aa";

/** Limit documents by degree centrality (highest-connected first). */
export function limitVaultDocsByDegree(
  docs: VaultDocument[],
  links: VaultLink[],
  nodeLimit: number,
): VaultDocument[] {
  if (docs.length <= nodeLimit) return docs;
  const ids = new Set(docs.map((d) => d.id));
  const deg = new Map<string, number>();
  for (const l of links) {
    if (ids.has(l.from_doc_id)) deg.set(l.from_doc_id, (deg.get(l.from_doc_id) ?? 0) + 1);
    if (ids.has(l.to_doc_id)) deg.set(l.to_doc_id, (deg.get(l.to_doc_id) ?? 0) + 1);
  }
  return [...docs].sort((a, b) => (deg.get(b.id) ?? 0) - (deg.get(a.id) ?? 0)).slice(0, nodeLimit);
}

/** Build a Graphology graph from vault documents and their links. */
export function buildVaultGraph(
  documents: VaultDocument[],
  links: VaultLink[],
): Graph {
  const graph = new Graph({ multi: false, type: "directed" });
  const docIds = new Set(documents.map((d) => d.id));

  // Pre-compute degree map
  const degreeMap = new Map<string, number>();
  for (const link of links) {
    if (docIds.has(link.from_doc_id))
      degreeMap.set(link.from_doc_id, (degreeMap.get(link.from_doc_id) ?? 0) + 1);
    if (docIds.has(link.to_doc_id))
      degreeMap.set(link.to_doc_id, (degreeMap.get(link.to_doc_id) ?? 0) + 1);
  }

  // Add nodes (x/y assigned by container via circular layout before FA2)
  for (const doc of documents) {
    const degree = degreeMap.get(doc.id) ?? 0;
    const rawLabel = doc.title || doc.path.split("/").pop() || doc.id.slice(0, 8);
    graph.addNode(doc.id, {
      label: truncateMiddle(rawLabel, 28),
      x: 0,
      y: 0,
      size: getNodeSize(degree),
      color: VAULT_TYPE_COLORS[doc.doc_type] ?? DEFAULT_COLOR,
      docType: doc.doc_type,
    });
  }

  // Add edges (only where both endpoints exist)
  for (const link of links) {
    if (docIds.has(link.from_doc_id) && docIds.has(link.to_doc_id)) {
      // Avoid duplicate edges for same source→target
      if (!graph.hasEdge(link.from_doc_id, link.to_doc_id)) {
        graph.addEdgeWithKey(link.id, link.from_doc_id, link.to_doc_id, {
          label: link.link_type,
          type: "curvedArrow",
        });
      }
    }
  }

  return graph;
}
