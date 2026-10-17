# Source inventory

The primary sources for this repository-specific design are the checked-in implementation. [code-evidence.md](code-evidence.md) contains bounded line-numbered excerpts captured by [01-capture-code-evidence.py](../scripts/01-capture-code-evidence.py). The design guide provides the complete file map. No external sources or provider behavior were needed; no production credentials or user data were read.

Current facts: hosted grants are rechecked on requests; RPC is capability gated; page tools execute through a browser SSE round trip; RAG eval is browser new Function; download policy differs from public metadata visibility; worker tool count is 16. Proposed facts: the slop_boutique namespace, read service and automatic server tool injection do not exist yet.

The guide embeds two rendered PNG diagrams. Their editable Graphviz DOT and original Mermaid sources are saved alongside them; scripts/02-render-diagrams.sh reproduces the PNGs with installed Graphviz. Both PNGs were visually inspected for legibility and clipping. The final PDF image inventory is recorded in pdf-image-verification.txt.
