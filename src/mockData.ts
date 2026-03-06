import { Commit, FileChange, Branch } from './types';

export const mockBranches: Branch[] = [
  { name: 'main', isCurrent: true, remote: 'origin/main' },
  { name: 'feature/electron-ipc', isCurrent: false },
  { name: 'fix/sidebar-scroll', isCurrent: false },
  { name: 'chore/deps-update', isCurrent: false },
];

export const mockCommits: Commit[] = [
  {
    hash: 'a1b2c3d',
    message: "Merge branch 'feature/electron-ipc'",
    author: 'Alex Developer',
    date: '10 minutes ago',
    branches: ['main', 'origin/main'],
    parents: ['e4f5g6h', 'i7j8k9l'],
    graph: {
      color: '#F14E32',
      column: 0,
      connections: [
        { toColumn: 0, toRow: 1, color: '#F14E32' },
        { toColumn: 1, toRow: 2, color: '#14B8A6' }
      ]
    }
  },
  {
    hash: 'e4f5g6h',
    message: 'fix: resolve race condition in repo scanning',
    author: 'Sam Coder',
    date: '2 hours ago',
    parents: ['m0n1o2p'],
    graph: {
      color: '#F14E32',
      column: 0,
      connections: [
        { toColumn: 0, toRow: 3, color: '#F14E32' }
      ]
    }
  },
  {
    hash: 'i7j8k9l',
    message: 'feat: implement IPC bridge for git commands',
    author: 'Alex Developer',
    date: 'Yesterday',
    branches: ['feature/electron-ipc'],
    parents: ['m0n1o2p'],
    graph: {
      color: '#14B8A6',
      column: 1,
      connections: [
        { toColumn: 0, toRow: 3, color: '#14B8A6' }
      ]
    }
  },
  {
    hash: 'm0n1o2p',
    message: 'style: update sidebar navigation icons',
    author: 'Jordan Writer',
    date: '2 days ago',
    parents: ['q3r4s5t'],
    graph: {
      color: '#F14E32',
      column: 0,
      connections: [
        { toColumn: 0, toRow: 4, color: '#F14E32' }
      ]
    }
  },
  {
    hash: 'q3r4s5t',
    message: 'chore: setup golang backend structure',
    author: 'Alex Developer',
    date: '3 days ago',
    tags: ['v0.1.0'],
    parents: ['u6v7w8x'],
    graph: {
      color: '#F14E32',
      column: 0,
      connections: [
        { toColumn: 0, toRow: 5, color: '#F14E32' }
      ]
    }
  },
  {
    hash: 'u6v7w8x',
    message: 'Initial commit',
    author: 'Alex Developer',
    date: '4 days ago',
    parents: [],
    graph: {
      color: '#F14E32',
      column: 0,
      connections: []
    }
  }
];

export const mockFileChanges: FileChange[] = [
  {
    path: 'backend/main.go',
    status: 'modified',
    additions: 45,
    deletions: 12,
    patch: `@@ -15,6 +15,12 @@ func main() {
 \tfmt.Println("Starting Git Client Backend...")
 \t
 \t// Initialize IPC server
+\tipcServer := ipc.NewServer()
+\terr := ipcServer.Start()
+\tif err != nil {
+\t\tlog.Fatalf("Failed to start IPC server: %v", err)
+\t}
+
 \t// Setup Git bindings
 \tgit.Init()
 }`
  },
  {
    path: 'backend/ipc/server.go',
    status: 'added',
    additions: 120,
    deletions: 0,
    patch: `@@ -0,0 +1,15 @@
+package ipc
+
+import "fmt"
+
+type Server struct {
+    port int
+}
+
+func NewServer() *Server {
+    return &Server{port: 8080}
+}
+
+func (s *Server) Start() error {
+    return nil
+}`
  },
  {
    path: 'frontend/package.json',
    status: 'modified',
    additions: 2,
    deletions: 1,
    patch: `@@ -10,7 +10,8 @@
   "dependencies": {
     "react": "^18.2.0",
-    "react-dom": "^18.2.0"
+    "react-dom": "^18.2.0",
+    "lucide-react": "^0.263.1"
   }
 }`
  }
];
