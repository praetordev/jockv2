import React, { useState } from 'react';
import { FolderGit2, FolderOpen, Plus, Download } from 'lucide-react';
import { useAppContext } from '../context/AppContext';

export default function WelcomeScreen() {
  const { openRepo, createRepo, cloneRepo, setShowRemoteSetup } = useAppContext();
  const [cloneUrl, setCloneUrl] = useState('');

  return (
    <div className="flex-1 flex flex-col items-center justify-center bg-zinc-950 text-center">
      <FolderGit2 className="w-16 h-16 text-zinc-800 mb-4" />
      <h2 className="text-xl font-semibold text-zinc-400 mb-2">Welcome to Jock</h2>
      <p className="text-sm text-zinc-600 mb-6">Open a Git repository to get started</p>
      <div className="flex items-center gap-3 mb-6">
        <button
          onClick={openRepo}
          className="flex items-center px-5 py-2.5 rounded-md bg-[#F14E32] hover:bg-[#d4432b] text-white text-sm font-medium transition-colors"
        >
          <FolderOpen className="w-4 h-4 mr-2" />
          Open Repository
        </button>
        <button
          onClick={async () => {
            const path = await createRepo();
            if (path) {
              setShowRemoteSetup(true);
            }
          }}
          className="flex items-center px-5 py-2.5 rounded-md bg-[#F14E32] hover:bg-[#d4432b] text-white text-sm font-medium transition-colors"
        >
          <Plus className="w-4 h-4 mr-2" />
          Create Repository
        </button>
      </div>
      <form
        className="flex items-center gap-2"
        onSubmit={(e) => { e.preventDefault(); if (cloneUrl.trim()) cloneRepo(cloneUrl.trim()); }}
      >
        <input
          type="text"
          value={cloneUrl}
          onChange={(e) => setCloneUrl(e.target.value)}
          placeholder="https://github.com/user/repo.git"
          className="bg-zinc-900 border border-zinc-800 rounded-md px-3 py-2 text-sm focus:outline-none focus:border-[#F14E32] focus:ring-1 focus:ring-[#F14E32] w-80 transition-all"
        />
        <button
          type="submit"
          disabled={!cloneUrl.trim()}
          className="flex items-center px-5 py-2.5 rounded-md bg-[#F14E32] hover:bg-[#d4432b] text-white text-sm font-medium transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
        >
          <Download className="w-4 h-4 mr-2" />
          Clone
        </button>
      </form>
    </div>
  );
}
