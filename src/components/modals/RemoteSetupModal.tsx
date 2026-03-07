import { useState, useEffect } from 'react';
import { isElectron } from '../../lib/electron';
import AccessibleModal from '../AccessibleModal';

interface RemoteSetupModalProps {
  show: boolean;
  onClose: () => void;
  initialRepoName: string;
  onSuccess: () => void;
}

export default function RemoteSetupModal({ show, onClose, initialRepoName, onSuccess }: RemoteSetupModalProps) {
  const [step, setStep] = useState<'choice' | 'github' | 'manual'>('choice');
  const [error, setError] = useState<string | null>(null);
  const [ghRepoName, setGhRepoName] = useState(initialRepoName);
  const [ghPrivate, setGhPrivate] = useState(true);
  const [ghUser, setGhUser] = useState<string | null>(null);
  const [manualRemoteUrl, setManualRemoteUrl] = useState('');

  useEffect(() => {
    if (show) {
      setStep('choice');
      setError(null);
      setGhRepoName(initialRepoName);
      setGhPrivate(true);
      setManualRemoteUrl('');
      if (isElectron) {
        window.electronAPI.invoke('git:get-user-info').then((info: any) => {
          if (info?.ghUser) setGhUser(info.ghUser);
        });
      }
    }
  }, [show, initialRepoName]);

  const handleClose = () => {
    onClose();
    setStep('choice');
    setError(null);
  };

  return (
    <AccessibleModal show={show} onClose={handleClose} title="Add Remote" className="w-96">
      <h3 className="text-sm font-semibold text-zinc-100 mb-1">Add Remote</h3>
      <p className="text-xs text-zinc-500 mb-4">Connect your new repo to a remote</p>

      {step === 'choice' && (
        <div className="space-y-2">
          <button
            onClick={() => setStep('github')}
            className="w-full text-left px-3 py-2.5 rounded-md bg-zinc-800 hover:bg-zinc-700/80 transition-colors text-sm text-zinc-200"
          >
            <div className="font-medium">Create on GitHub</div>
            <div className="text-xs text-zinc-500 mt-0.5">Uses gh CLI to create a repo and set origin</div>
          </button>
          <button
            onClick={() => setStep('manual')}
            className="w-full text-left px-3 py-2.5 rounded-md bg-zinc-800 hover:bg-zinc-700/80 transition-colors text-sm text-zinc-200"
          >
            <div className="font-medium">Add remote URL</div>
            <div className="text-xs text-zinc-500 mt-0.5">Paste an existing remote URL as origin</div>
          </button>
          <button
            onClick={handleClose}
            className="w-full text-center px-3 py-2 text-xs text-zinc-500 hover:text-zinc-300 transition-colors"
          >
            Skip
          </button>
        </div>
      )}

      {step === 'github' && (
        <form
          onSubmit={async (e) => {
            e.preventDefault();
            if (!ghRepoName.trim()) return;
            setError(null);
            const btn = e.currentTarget.querySelector('button[type="submit"]') as HTMLButtonElement;
            if (btn) { btn.disabled = true; btn.textContent = 'Creating...'; }
            const result = await window.electronAPI.invoke('git:create-github-repo', ghRepoName.trim(), ghPrivate);
            if (btn) { btn.disabled = false; btn.textContent = 'Create on GitHub'; }
            if (result.success) {
              handleClose();
              onSuccess();
            } else {
              setError(result.error);
            }
          }}
          className="space-y-3"
        >
          {ghUser && (
            <div className="text-xs text-zinc-400">Signed in as <span className="text-zinc-200 font-medium">{ghUser}</span></div>
          )}
          <div>
            <label className="text-xs text-zinc-400 block mb-1">Repository name</label>
            <div className="flex items-center bg-zinc-800 border border-zinc-700 rounded-md overflow-hidden focus-within:border-[#F14E32] focus-within:ring-1 focus-within:ring-[#F14E32]">
              {ghUser && <span className="text-xs text-zinc-500 pl-3 flex-shrink-0">{ghUser}/</span>}
              <input
                type="text"
                value={ghRepoName}
                onChange={(e) => setGhRepoName(e.target.value)}
                className="flex-1 bg-transparent px-3 py-1.5 text-sm focus:outline-none"
                autoFocus
              />
            </div>
          </div>
          <label className="flex items-center gap-2 text-xs text-zinc-400 cursor-pointer">
            <input type="checkbox" checked={ghPrivate} onChange={(e) => setGhPrivate(e.target.checked)} className="rounded" />
            Private repository
          </label>
          {error && (
            <div role="alert" className="text-xs text-red-400 bg-red-400/10 border border-red-400/20 rounded-md p-2 break-words">{error}</div>
          )}
          <div className="flex gap-2">
            <button
              type="submit"
              className="flex-1 px-3 py-2 rounded-md bg-[#F14E32] hover:bg-[#d4432b] text-white text-sm font-medium transition-colors disabled:opacity-50"
            >
              Create on GitHub
            </button>
            <button
              type="button"
              onClick={() => { setStep('choice'); setError(null); }}
              className="px-3 py-2 rounded-md text-sm text-zinc-400 hover:text-zinc-200 transition-colors"
            >
              Back
            </button>
          </div>
        </form>
      )}

      {step === 'manual' && (
        <form
          onSubmit={async (e) => {
            e.preventDefault();
            if (!manualRemoteUrl.trim()) return;
            setError(null);
            const result = await window.electronAPI.invoke('git:add-remote', manualRemoteUrl.trim());
            if (result.success) {
              setManualRemoteUrl('');
              handleClose();
              onSuccess();
            } else {
              setError(result.error);
            }
          }}
          className="space-y-3"
        >
          {error && (
            <div role="alert" className="text-xs text-rose-400 bg-rose-500/10 px-3 py-2 rounded">{error}</div>
          )}
          <div>
            <label className="text-xs text-zinc-400 block mb-1">Remote URL</label>
            <input
              type="text"
              value={manualRemoteUrl}
              onChange={(e) => setManualRemoteUrl(e.target.value)}
              placeholder="https://github.com/user/repo.git"
              className="w-full bg-zinc-800 border border-zinc-700 rounded-md px-3 py-1.5 text-sm focus:outline-none focus:border-[#F14E32] focus:ring-1 focus:ring-[#F14E32]"
              autoFocus
            />
          </div>
          <div className="flex gap-2">
            <button
              type="submit"
              className="flex-1 px-3 py-2 rounded-md bg-[#F14E32] hover:bg-[#d4432b] text-white text-sm font-medium transition-colors"
            >
              Add Remote
            </button>
            <button
              type="button"
              onClick={() => { setStep('choice'); setError(null); }}
              className="px-3 py-2 rounded-md text-sm text-zinc-400 hover:text-zinc-200 transition-colors"
            >
              Back
            </button>
          </div>
        </form>
      )}
    </AccessibleModal>
  );
}
