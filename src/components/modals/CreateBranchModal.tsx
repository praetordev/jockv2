import { useState, useEffect } from 'react';
import AccessibleModal from '../AccessibleModal';

interface CreateBranchModalProps {
  show: boolean;
  onClose: () => void;
  onSuccess: () => void;
  doCreateBranch: (name: string, checkout: boolean) => Promise<{ success?: boolean } | undefined>;
  creatingBranch: boolean;
}

export default function CreateBranchModal({ show, onClose, onSuccess, doCreateBranch, creatingBranch }: CreateBranchModalProps) {
  const [newBranchName, setNewBranchName] = useState('');

  useEffect(() => {
    if (show) setNewBranchName('');
  }, [show]);

  return (
    <AccessibleModal show={show} onClose={onClose} title="Create Branch">
      <h3 className="text-sm font-semibold text-zinc-100 mb-3">Create Branch</h3>
      <form
        onSubmit={async (e) => {
          e.preventDefault();
          if (!newBranchName.trim()) return;
          const result = await doCreateBranch(newBranchName.trim(), true);
          if (result?.success) {
            onClose();
            onSuccess();
          }
        }}
        className="space-y-3"
      >
        <input
          type="text"
          value={newBranchName}
          onChange={(e) => setNewBranchName(e.target.value)}
          placeholder="Branch name..."
          className="w-full bg-zinc-800 border border-zinc-700 rounded-md px-3 py-1.5 text-sm focus:outline-none focus:border-[#F14E32] focus:ring-1 focus:ring-[#F14E32]"
          autoFocus
        />
        <div className="flex gap-2">
          <button
            type="submit"
            disabled={!newBranchName.trim() || creatingBranch}
            className="flex-1 px-3 py-2 rounded-md bg-[#F14E32] hover:bg-[#d4432b] text-white text-sm font-medium transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
          >
            {creatingBranch ? 'Creating...' : 'Create & Switch'}
          </button>
          <button
            type="button"
            onClick={onClose}
            className="px-3 py-2 rounded-md text-sm text-zinc-400 hover:text-zinc-200 transition-colors"
          >
            Cancel
          </button>
        </div>
      </form>
    </AccessibleModal>
  );
}
