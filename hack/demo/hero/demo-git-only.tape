# Hero demo — Git-only (≤60s README teaser). Run: task demo-hero-record-git-only
Output docs/assets/demo/hero-git-only.gif
Output docs/assets/demo/hero-git-only.mp4

Set Shell "bash"
Set FontSize 12
Set Width 1200
Set Height 680
Set Theme "Catppuccin Mocha"
Set TypingSpeed 40ms
Set PlaybackSpeed 1.0

# Scene 1 — title card (~2s)
Type "# kollect — your cluster, in Git, diffable"
Sleep 2s
Ctrl+L

# Scene 2 — one-command bootstrap (~8s; cluster should already be up for recording)
Type "task demo-hero-up"
Enter
Sleep 2s
Hide
Sleep 6s
Show
Sleep 1s
Ctrl+L

# Scene 3 — config is just CRs (~8s)
Type "bat hack/demo/hero/kollect-demo.yaml"
Enter
Sleep 8s
Ctrl+L

# Scene 4 — inventory Ready (~5s)
Type "kubectl get kinv -A"
Enter
Sleep 5s
Ctrl+L

# Scene 5 — first export (~10s)
Type "git -C /tmp/kollect-hero-inventory pull && ls -R /tmp/kollect-hero-inventory/namespaces/demo/"
Enter
Sleep 3s
Type "bat /tmp/kollect-hero-inventory/namespaces/demo/deployment/shop.yaml"
Enter
Sleep 7s
Ctrl+L

# Scene 6 — trigger drift (~4s)
Type "kubectl set image deployment/shop api=shop:v2.1.0 -n demo"
Enter
Sleep 4s
Ctrl+L

# Scene 7 — money shot: git log -p (~12s)
Hide
Sleep 20s
Show
Type "git -C /tmp/kollect-hero-inventory pull && git -C /tmp/kollect-hero-inventory log -p -1"
Enter
Sleep 12s
Ctrl+L

# Scene 8 — close (~4s)
Type "# git log = audit trail · git diff = drift report"
Enter
Sleep 1s
Type "# https://platformrelay.github.io/Kollect/"
Sleep 3s
