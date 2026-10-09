---
name: remind
description: Read the latest handoff entry and quote its origin.
---

Call the recall_handoff tool. Print its stdout unchanged.

origin.harness is the harness that wrote the entry. Do not describe the entry as written by the current harness. If the tool prints `no handoff entry`, say that and stop.

This slash command still starts an agent turn. The tool command is the deterministic read. Hermes `/remind` runs `handoff show` without a model.
