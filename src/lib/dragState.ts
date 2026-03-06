// Shared drag payload — bypasses dataTransfer API which can be unreliable in Electron.
// Set during dragstart, consumed (and cleared) in drop.
export type DragPayload =
  | { type: 'file'; filePath: string; sourceBranchId: string | null }
  | { type: 'hunk'; filePath: string; hunkIndex: number; sourceBranchId: string };

export const dragPayload: { current: DragPayload | null } = { current: null };
