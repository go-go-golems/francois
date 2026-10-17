"""Generate report figures from the retained measurements; no app dependencies."""
import json
from pathlib import Path
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt

ticket = Path(__file__).resolve().parents[1]
imports = ticket.parent / 'FRANCOIS-009--import-real-html-artifacts-as-hosted-proofs-of-concept'
metrics = json.loads((imports / 'various/web-media-optimization.json').read_text())
plt.rcParams.update({'font.family': 'DejaVu Sans', 'font.size': 11})
fig, ax = plt.subplots(figsize=(9, 3.8), layout='constrained')
values = [metrics['pngBytes'] / 1e6, metrics['webpBytes'] / 1e6]
bars = ax.barh(['Original report PNGs', 'Published WebP copies'], values, color=['#727b86', '#247a57'], height=0.55)
ax.invert_yaxis()
ax.set_xlim(0, 12)
ax.set_xlabel('Encoded file size, decimal MB — all 17 selected images')
ax.set_title('Screenshot delivery: 81.3% fewer bytes at original dimensions', loc='left', weight='bold', pad=18)
for bar, value in zip(bars, values):
    ax.text(value + 0.12, bar.get_y() + bar.get_height()/2, f'{value:.2f} MB', va='center')
ax.spines[['top', 'right', 'left']].set_visible(False)
ax.tick_params(axis='y', length=0)
ax.grid(axis='x', alpha=0.15)
ax.set_axisbelow(True)
fig.savefig(ticket / 'various/media-transfer.png', dpi=180)
plt.close(fig)

after_file = ticket / 'various/cache-after.json'
if after_file.exists():
    before = json.loads((ticket / 'various/cache-before.json').read_text())
    after = json.loads(after_file.read_text())
    routes = ['/apps', '/apps/lattice-deck', '/account']
    def totals(receipt):
        return [sum(r.get('wireBytes', 0) for r in next(v for v in receipt['visits'] if v['route'] == route and v['pass'] == 'warm')['responses']) / 1000 for route in routes]
    fig, ax = plt.subplots(figsize=(9, 4.5), layout='constrained')
    x = list(range(len(routes)))
    ax.bar([i - .18 for i in x], totals(before), .36, label='Before', color='#727b86')
    ax.bar([i + .18 for i in x], totals(after), .36, label='After', color='#247a57')
    ax.set_xticks(x, routes)
    ax.set_ylabel('Observed response wire bytes, decimal KB')
    ax.set_title('Ordinary warm navigation: measured response transfer', loc='left', weight='bold')
    ax.legend(frameon=False)
    ax.spines[['top', 'right']].set_visible(False)
    ax.grid(axis='y', alpha=.15)
    ax.set_axisbelow(True)
    fig.savefig(ticket / 'various/warm-transfer.png', dpi=180)
    plt.close(fig)
