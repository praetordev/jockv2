import { useState, useEffect } from 'react';
import AccessibleModal from '../AccessibleModal';

interface CreateTagModalProps {
  show: boolean;
  onClose: () => void;
  onSuccess: () => void;
  doCreateTag: (tagName: string, commitHash?: string, message?: string) => Promise<any>;
  defaultCommitHash?: string;
}

export default function CreateTagModal({
  show,
  onClose,
  onSuccess,
  doCreateTag,
  defaultCommitHash,
}: CreateTagModalProps) {
  const [tagName, setTagName] = useState('');
  const [commitHash, setCommitHash] = useState('');
  const [message, setMessage] = useState('');
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    if (show) {
      setTagName('');
      setMessage('');
      setCommitHash(defaultCommitHash || '');
    }
  }, [show, defaultCommitHash]);

  return (
    <AccessibleModal show={show} onClose={onClose} title="Create Tag">
      <h3 className="text-sm font-semibold text-zinc-100 mb-3">Create Tag</h3>
      <form
        onSubmit={async (e) => {
          e.preventDefault();
          if (!tagName.trim()) return;
          setCreating(true);
          try {
            const result = await doCreateTag(tagName.trim(), commitHash || undefined, message || undefined);
            if (result?.success) {
              onClose();
              onSuccess();
            }
          } finally {
            setCreating(false);
          }
        }}
        className="space-y-3"
      >
        <input
          type="text"
          value={tagName}
          onChange={(e) => setTagName(e.target.value)}
          placeholder="Tag name (e.g. v1.0.0)"
          className="w-full bg-zinc-800 border border-zinc-700 rounded-md px-3 py-1.5 text-sm focus:outline-none focus:border-[#F14E32] focus:ring-1 focus:ring-[#F14E32]"
          autoFocus
        />
        <input
          type="text"
          value={commitHash}
          onChange={(e) => setCommitHash(e.target.value)}
          placeholder="Commit (default: HEAD)"
          className="w-full bg-zinc-800 border border-zinc-700 rounded-md px-3 py-1.5 text-sm text-zinc-400 focus:outline-none focus:border-[#F14E32] focus:ring-1 focus:ring-[#F14E32] font-mono"
        />
        <textarea
          value={message}
          onChange={(e) => setMessage(e.target.value)}
          placeholder="Message (optional — creates annotated tag)"
          rows={2}
          className="w-full bg-zinc-800 border border-zinc-700 rounded-md px-3 py-1.5 text-sm focus:outline-none focus:border-[#F14E32] focus:ring-1 focus:ring-[#F14E32] resize-none"
        />
        <div className="flex gap-2">
          <button
            type="submit"
            disabled={!tagName.trim() || creating}
            className="flex-1 px-3 py-2 rounded-md bg-[#F14E32] hover:bg-[#d4432b] text-white text-sm font-medium transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
          >
            {creating ? 'Creating...' : 'Create Tag'}
          </button>
          <button
            type="button"
            onClick={onClose}
            className="px-3 py-2 rounded-md text-sm text-zinc-400 hover:text-zinc-200"
          >
            Cancel
          </button>
        </div>
      </form>
    </AccessibleModal>
  );
}
