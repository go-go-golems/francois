# Hide the public blog by default behind an admin storefront setting

This is the document workspace for ticket FRANCOIS-016.

## Structure

- **design/**: Design documents and architecture notes
- **reference/**: Reference documentation and API contracts
- **playbooks/**: Operational playbooks and procedures
- **scripts/**: Utility scripts and automation
- **sources/**: External sources and imported documents
- **various/**: Scratch or meeting notes, working notes
- **archive/**: Optional space for deprecated or reference-only artifacts

## Getting Started

Use docmgr commands to manage this workspace:

- Add documents: `docmgr doc add --ticket FRANCOIS-016 --doc-type design-doc --title "My Design"`
- Import sources: `docmgr import file --ticket FRANCOIS-016 --file /path/to/doc.md`
- Update metadata: `docmgr meta update --ticket FRANCOIS-016 --field Status --value review`
