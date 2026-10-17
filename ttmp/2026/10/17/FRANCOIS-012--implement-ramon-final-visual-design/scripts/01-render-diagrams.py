from pathlib import Path
import subprocess

out = Path(__file__).resolve().parent.parent / 'sources'
out.mkdir(exist_ok=True)
graphs = {
    '01-system': '''digraph G {
      rankdir=TB;
      node [shape=box, style=rounded, fontname="DejaVu Sans", fontsize=13];
      edge [fontname="DejaVu Sans", fontsize=11];
      browser [label="Visitor browser"];
      go [label="Go public pages\nTemplates + small public script"];
      react [label="React account / staff pages\nSeparate shared shell"];
      db [label="Existing PostgreSQL\nArtifacts, releases, sessions, instances"];
      run [label="Isolated instance origin\nExisting iframe runtime"];
      browser -> go [label="archive / experiment"];
      browser -> react [label="account / staff"];
      go -> db; react -> db;
      go -> run;
      react -> run;
    }''',
    '02-runner': '''digraph G {
      rankdir=TB;
      node [shape=box, style=rounded, fontname="DejaVu Sans", fontsize=13];
      edge [fontname="DejaVu Sans", fontsize=11];
      preview [label="Preview: cover + Start\nNo instance allocated"];
      start [label="User presses Start\nCreate or resume existing visit"];
      session [label="Refresh session after guest creation\nResolve instance origin"];
      frame [label="Load isolated /launch iframe"];
      proof [label="Validate exact frame + origin\nExchange challenge for launch code"];
      live [label="Live experiment\nReload / Full screen"];
      denied [label="Server denied / visit expired\nShow existing service message"];
      preview -> start -> session -> frame -> proof -> live;
      start -> denied; proof -> denied;
      live -> frame [label="Reload reuses same instance"];
    }''',
}
for name, dot in graphs.items():
    source = out / (name + '.dot')
    source.write_text(dot)
    subprocess.run(['dot', '-Tpng', '-Gdpi=130', str(source), '-o', str(out / (name + '.png'))], check=True)
print('Rendered two visual-guide diagrams')
