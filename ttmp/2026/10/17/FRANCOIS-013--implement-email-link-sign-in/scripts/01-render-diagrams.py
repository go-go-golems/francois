from pathlib import Path
import subprocess

out = Path(__file__).resolve().parent.parent / 'sources'
out.mkdir(exist_ok=True)
graphs = {
    '01-system': '''digraph G {
      rankdir=TB;
      node [shape=box, style=rounded, fontname="DejaVu Sans", fontsize=13];
      edge [fontname="DejaVu Sans", fontsize=11];
      browser [label="Subscriber browser\nFinal email form + binding cookie"];
      go [label="Existing Go service\nRequest / confirmation handlers"];
      db [label="Existing PostgreSQL\nUsers + sessions + one pending-link table"];
      smtp [label="Configured SMTP relay\nOne bounded submission"];
      mail [label="Subscriber mailbox\n10-minute single-use link"];
      browser -> go; go -> db;
      go -> smtp -> mail;
      mail -> browser [label="Open in requesting browser"];
    }''',
    '02-flow': '''digraph G {
      rankdir=TB;
      node [shape=box, style=rounded, fontname="DejaVu Sans", fontsize=13];
      edge [fontname="DejaVu Sans", fontsize=11];
      form [label="GET /signin\nCreate short-lived browser binding"];
      request [label="POST /auth/email/request\nValidate nonce + origin + quotas"];
      pending [label="Resolve existing subscriber\nStore token digest; submit email"];
      ack [label="Same generic acknowledgement\nNo account created"];
      inspect [label="GET emailed verification link\nShow Continue; do not consume"];
      consume [label="POST Continue with binding\nOne database transaction"];
      commit [label="Delete token + validate user\nInsert normal session + commit"];
      redirect [label="Set secure session cookie\n303 to stored local return path"];
      form -> request -> pending -> ack;
      pending -> inspect [label="email received"];
      inspect -> consume -> commit -> redirect;
    }''',
}
for name, dot in graphs.items():
    source = out / (name + '.dot')
    source.write_text(dot)
    subprocess.run(['dot', '-Tpng', '-Gdpi=130', str(source), '-o', str(out / (name + '.png'))], check=True)
print('Rendered two email-guide diagrams')
