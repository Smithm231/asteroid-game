# DRIFTHOLD — Game & Simulation Specification (v1)

Drifthold is an ORIGINAL real-time colony-building strategy game for Windows 11 (native .exe),
written in Go with Ebitengine v2.10.6. Repository root: `/home/user/drifthold` (module `drifthold`).

## 0. Ground rules for every contributor

* **Originality.** Everything here is an original design. Never reference, imitate or borrow names,
  art, text, UI layout or data from any existing commercial game. Do not mention other games in
  code, comments, docs or commit messages. Use only the names defined in this spec.
* **Work only inside `/home/user/drifthold`.** Never touch `/home/user/Pc-alt-ai-mode`.
  Do NOT run `git commit`/`git add`/any git write command — the coordinator commits.
* **Own your files.** Each task names the package directory you own. Create/modify files only
  there. Every other package is read-only for you. If you need something that another package
  lacks, implement a private helper in your own package and list it under "wishlist" in your final
  report — do not edit the other package.
* **Determinism.** The simulation (`internal/model` and every system package) must be fully
  deterministic: no `time.Now`, no `math/rand` globals, no goroutines, no iteration over Go maps
  whose order affects results (sort keys first). All randomness comes from `w.RNG`.
  The same seed + the same command stream must produce byte-identical saves.
* **Plain data.** Everything reachable from `model.World` must round-trip through
  `encoding/json` (no funcs, channels, interfaces, cyclic pointers, or maps with non-string keys).
  Cross references use IDs, never pointers.
* **Sim packages never import Ebitengine** (model, worldgen, economy, construct, fleet, combat,
  research, events, market, directives, ai, sim, save). They must build with `CGO_ENABLED=0`.
* **Quality bar.** `gofmt`-clean, `go vet` clean, meaningful table-driven tests in each package,
  doc comments on every exported identifier. No TODO stubs, no placeholder logic, no panics on bad
  input from commands (return errors instead).
* **Verify APIs against source, not memory.** Third-party source lives in
  `$(go env GOMODCACHE)` (e.g. `github.com/hajimehoshi/ebiten/v2@v2.10.6`). Read it when unsure.
  Avoid deprecated Ebitengine APIs.
* Build/test only your own package (`go build ./internal/<pkg>/... && go vet ./internal/<pkg>/... &&
  go test ./internal/<pkg>/...`). Other packages may be mid-edit by other contributors; if a
  compile error is in a file you do not own, do not fix it — re-run a little later or report it.

## 1. Fiction (all original)

The **Shoal** is a ring of shattered-planet debris circling the red dwarf **Emberfall**. Chartered
mining companies — **Holds** — stake claims on the drifting **rocks**. The player is the Warden of
a newly chartered Hold. Rival Holds compete for the same rocks; there is no diplomacy, every Hold
is hostile to every other Hold. A Hold exists while it controls at least one **Keystone** (the
command structure of a colony). Destroy every rival Keystone to win.

Rival Hold personalities (AI): **Ostrander Deepworks** (industrial turtle: strong economy and
defences, late heavy fleets), **The Vane Compact** (raiders: early, frequent light attacks),
**Meridian Lattice** (expansionist: claims many rocks, medium fleets).
Default player Hold name: **Free Charter of Ansel**. Faction accent colours (index): 0 neutral
grey, 1 amber (player), 2 teal, 3 magenta, 4 lime.

## 2. Time & units

* `TicksPerSecond = 10`. One sim tick = 0.1 game-seconds. One **cycle** = 60 game-seconds = 600 ticks.
  Displayed date: `Cycle = Tick/600 + 1`.
* All rates in content tables are **per cycle** unless stated. Build times are in game-seconds.
* Game speed (pause / 1× / 2× / 4×) is purely an app-level matter: the app calls `sim.Step` more
  often. The sim has no notion of speed.
* Belt space uses abstract **units (u)**, Emberfall at (0,0). Angles in radians.
* Cadence: movement & combat run every tick (dt = 0.1 s). Economy, construction, research, market,
  events, directives, victory checks run when `w.Tick % 10 == 0` with dt = 1.0 s. AI runs when
  `w.Tick % 20 == 0` per faction (staggered by faction id).

## 3. The belt

* 10 / 13 / 16 rocks for Small / Medium / Large belts. Orbit radius 380..950 u, circular orbits,
  all prograde. Angular speed `ω(r) = 2π / (1080 s × (r/400)^1.5)` rad per game-second
  (r=400 → 18-minute period). Position at tick t: `angle = Phase0 + ω·t/10`,
  `pos = (r·cos, r·sin)`. Rocks never collide (orbits at least 35 u apart radially, or well
  separated in phase).
* **Solar factor** of a rock: `clamp((520/r)^1.2, 0.45, 1.6)`.
* Rock classes: **Ferric** (Ore-rich; common inside r<650), **Stony** (Silica-rich, some Ore),
  **Frost** (Ice-rich; only r>560), **Glint** (small, Lumen-bearing; rare, 2–3 per belt).
  Every faction's start rock is Stony or Ferric with guaranteed deposits near its Keystone:
  ≥1 Ore vein and ≥1 Silica vein within 4 hexes, and one small Ice pocket within 6 hexes.
  Start rocks are spread roughly evenly in phase, at mid radii.
* Each rock is a **hex map** (axial coords `Hex{Q,R}`, pointy-top; neighbour directions
  `(+1,0),(+1,-1),(0,-1),(-1,0),(-1,+1),(0,+1)`; distance `(|dq|+|dr|+|dq+dr|)/2`).
  Rock `Radius` is 7..11 hexes; tiles for every hex with distance ≤ Radius from (0,0) are stored
  in a dense slice, index `(R+Radius)*(2*Radius+1) + (Q+Radius)`. The actual rock is an irregular
  connected blob (≈60–75 % of the disc) — hexes outside the blob have `Terrain = Void`.
* Terrain: `Void` (not rock), `Regolith` (buildable), `Crag` (blocked ridge), `Fissure` (blocked
  crack). 10–18 % of blob tiles are Crag/Fissure, in small clusters, never enclosing unreachable
  regolith pockets near the centre.
* Deposits: a tile may hold `Deposit` ∈ {Ore, Ice, Silica, Lumen} with `Amount` units. Veins are
  clusters of 3–7 adjacent tiles, 350–1400 units per tile (Lumen: 1–3 tiles, 80–220 units each).
  Deposits are hidden per faction: `Tile.Known` is a bitmask (bit = faction id) of factions that
  have revealed the tile. Deposits can sit under any non-void terrain.
* `Tile.Rubble` marks wreckage of a destroyed building (cosmetic; building over it clears it and
  grants +10 Alloy).

## 4. Goods, stock, storage

Goods (enum order is fixed): `Ore, Ice, Silica, Lumen` (raw, mined) and `Alloy, Fuel, Parts, Food`
(produced). `NumGoods = 8`. `type Stock [NumGoods]float64`.

* Each colony (one per owned rock) has its own `Stock`. **Scrip** (currency) is per faction, global.
* Storage capacity is the same number for every good: `400 (Keystone) + 300 per active Silo`.
  Anything that would exceed capacity is not produced / not unloaded / not bought.
* Base market value per unit (Scrip): Ore 1.0, Ice 1.2, Silica 0.8, Lumen 12, Alloy 3.0, Fuel 3.0,
  Parts 9.0, Food 1.5.

## 5. Colonists, staffing, power, morale

All evaluated per colony each economy step (dt = 1 s), in this order:

1. **Capacity caches**: housing = 12 (Keystone) + 16 per active Habitat; life-support capacity =
   12 (Keystone) + 24 × Eff per Aerator; storage cap; berths.
2. **Staffing**: workers = floor(Pop). Each enabled, completed building needs
   `ceil(Staff × faction.Mods.StaffMul)` workers (0 stays 0). Sort by (Priority desc, category rank
   asc, ID asc) — category rank: Life, Power, Industry, Science, Space, Defence, Core — and assign
   workers until exhausted. `Staffed = assigned/needed` (1 if none needed).
3. **Power**: producers output `Power × Staffed × modifiers` (Sunvane: × rock Solar × SolarMul ×
   flare bonus; Thermal Stack: only while it can burn its Fuel; Lumen Core: only while it can burn
   Lumen). Keystone always supplies its 20. Consumers are enabled, completed buildings with
   negative Power. If supply ≥ demand all are `Powered`. Otherwise walk consumers in the same
   priority order, powering each fully while supply remains; the rest are unpowered.
4. **Efficiency**: `Eff = Powered × Staffed × moraleFactor` for consumers
   (moraleFactor: 1.10 if Morale ≥ 70, 1.0 if 30..70, 0.75 if < 30). Power producers use
   `Eff = Staffed`.
5. **Production** (see table) scaled by `Eff × dt/60 × tech multipliers`, limited by available
   inputs and by free storage for outputs (if either is lacking the building idles for the step and
   consumes nothing).
6. **Life support**: colonists eat 0.5 Food and use 0.2 Ice per colonist per cycle.
   `Starving` = Food would go below 0 (clamp to 0). `Suffocating` = Ice at 0 or life-support
   capacity < floor(Pop).
7. **Morale** (0..100) drifts toward a target at 0.5 points per second. Target = 55
   +10 if fed and breathing; −25 if Starving; −30 if Suffocating;
   −20 × clamp((Pop − housing)/housing, 0, 1) for overcrowding;
   + min(20, 20 × commonsCoverage) where commonsCoverage = (40 × Σ Commons Eff) / max(Pop,1) capped 1;
   −15 if the colony was attacked (any building damaged by an enemy) within the last 2 cycles.
8. **Population** (float): if Morale ≥ 45, not Starving, not Suffocating and Pop < housing:
   growth per cycle = max(0.5, 0.04 × Pop) × (Morale/60). If Starving or Suffocating:
   loss per cycle = 0.5 + 0.05 × Pop each. If Morale < 20: additional loss 0.02 × Pop per cycle.
   If Pop > housing × 1.25 no growth. Pop never below 0.
9. **Stipend**: faction Scrip += 0.4 × Pop per cycle.

Player-settable per building: `Enabled` (off = no power, no staff, no effect) and `Priority`
(0 low, 1 normal, 2 high; default 1, defence and life buildings default 2).

## 6. Buildings

Placement rules (`construct.CanPlace`): the rock is owned by the faction (it has that faction's
Keystone); the tile is Regolith with no building; the tile is adjacent to an existing building
of that faction on that rock (any state, Conduits count); the kind is unlocked for the faction;
the colony stock covers the cost; per-rock limits: Keystone 1 (never player-buildable),
Exchange Dock 1, Launch Cradle 3. Cost is deducted on placement. Construction progresses at
`1/BuildTime` per second (×0.5 while the colony is in power deficit). Cancelling a building under
construction refunds 100 %; demolishing a completed one refunds 50 % of Alloy/Silica/Parts (capped
by storage). A building at 0 HP is destroyed: removed, its tile gets `Rubble`.
Completed buildings below max HP self-repair at 0.5 HP/s when the colony has not been attacked for
1 cycle (Mender Hubs are much faster and work during attacks).

Costs: A = Alloy, S = Silica, P = Parts, L = Lumen. Power: + produces, − consumes.
Cat = category (palette tab & priority rank).

| Kind (const) | Display name | Cat | Cost | Build s | Power | Staff | HP | Unlock | Function |
|---|---|---|---|---|---|---|---|---|---|
| Keystone | Keystone | Core | — | — | +20 | 0 | 1200 | — | Colony command. Housing 12, life support 12, storage 400, 4 berths, sensor 260 u, reveals deposits within 3 hexes on founding. Its loss ends the colony. |
| Conduit | Conduit | Core | A4 | 3 | 0 | 0 | 60 | start | Cheap connector tile for extending the buildable area. |
| Silo | Silo | Core | A30 S10 | 15 | −1 | 0 | 300 | start | +300 storage for every good. |
| SurveyMast | Survey Mast | Core | A15 S5 | 12 | −2 | 1 | 100 | start | Reveals deposits within 4 hexes (+bonus), one nearest hidden tile every 1.5 s × Eff. |
| Sunvane | Sunvane | Power | A20 S15 | 15 | +12 × solar | 0 | 120 | start | Solar collector. |
| ThermalStack | Thermal Stack | Power | A60 S20 P10 | 30 | +45 | 2 | 250 | Volatile Cracking | Burns 6 Fuel/cycle. |
| LumenCore | Lumen Core | Power | A150 P60 L20 | 60 | +160 | 4 | 400 | Lumen Fusion | Burns 0.5 Lumen/cycle. |
| Habitat | Habitat | Life | A30 S30 | 20 | −3 | 0 | 200 | start | Housing 16. |
| Aerator | Aerator | Life | A25 S10 | 18 | −5 | 1 | 150 | start | Life support for 24 colonists. |
| Greenhouse | Greenhouse | Life | A20 S40 | 25 | −6 | 2 | 140 | start | +14 Food/cycle, uses 2 Ice/cycle. |
| Commons | Commons | Life | A40 S40 P5 | 30 | −4 | 2 | 180 | Civic Planning | Morale: covers 40 colonists. |
| BoreRig | Bore Rig | Industry | A35 | 20 | −8 | 3 | 220 | start | Mines 36 units/cycle total from deposits on its own tile and the 6 neighbours (split evenly between the deposit tiles that still have material; several rigs may share a tile). |
| Smelter | Smelter | Industry | A50 S20 | 30 | −14 | 3 | 260 | start | 30 Ore → 20 Alloy per cycle. |
| Cracker | Cracker | Industry | A45 S15 P5 | 30 | −10 | 2 | 220 | Volatile Cracking | 20 Ice → 16 Fuel per cycle. |
| Fabricator | Fabricator | Industry | A70 S30 | 40 | −16 | 4 | 260 | Fabrication | 12 Alloy + 8 Silica → 8 Parts per cycle. |
| Laboratory | Laboratory | Science | A60 S40 P10 | 45 | −12 | 4 | 200 | start | +6 research points/cycle. |
| LaunchCradle | Launch Cradle | Space | A120 S40 P30 | 60 | −15 | 4 | 400 | Orbital Flight | Shipyard with a build queue (max 5 queued). |
| DockRing | Dock Ring | Space | A80 P20 | 40 | −6 | 2 | 300 | Dock Engineering | +6 berths; friendly ships in orbit repair 2 HP/s per ring × Eff. |
| ExchangeDock | Exchange Dock | Space | A60 S30 P15 | 40 | −5 | 2 | 250 | Charter Trade | Enables market trading and settler sponsorship at this rock. |
| FarseerArray | Farseer Array | Space | A50 S30 P15 | 35 | −10 | 2 | 150 | Deep Scan | Sensor range 720 u. |
| MenderHub | Mender Hub | Defence | A60 P20 | 35 | −8 | 2 | 200 | Field Repair | Repairs damaged friendly buildings within 4 hexes at 6 HP/s × Eff each, 0.02 Alloy per HP. |
| FlakBattery | Flak Battery | Defence | A60 P10 | 30 | −6 | 2 | 350 | Point Defence | 8 dps vs ships in orbit. ×1.5 vs Light hulls, ×0.6 vs Heavy. |
| LanceTower | Lance Tower | Defence | A90 P30 L10 | 45 | −20 | 3 | 400 | Lumen Optics | 20 dps vs ships. ×0.5 vs Light, ×1.5 vs Heavy. |
| AegisProjector | Aegis Projector | Defence | A100 P40 L15 | 50 | −30 | 3 | 300 | Aegis Fields | Shield pool 600 (regen 10/s × Eff when powered) absorbs all damage to friendly buildings within 3 hexes (itself included). |
| MassDriver | Mass Driver | Defence | A140 P50 | 60 | −25 | 4 | 350 | Mass Acceleration | Bombards a designated enemy rock within 600 u: one slug per 15 s (8 Alloy each), 150 damage to one building. |

Starting unlocked: Conduit, Silo, Survey Mast, Sunvane, Habitat, Aerator, Greenhouse, Bore Rig,
Smelter, Laboratory.

## 7. Ships & fleets

| Kind | Display | Hull | Cost | Build s | Berths | HP | Speed u/s | dps vs ships | dps vs buildings | Fuel per u | Unlock | Role |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| Prospector | Prospector | Light | A30 P10 | 25 | 1 | 60 | 9 | 0 | 0 | 0.010 | Orbital Flight | Surveys rocks (reveals every deposit); sensor 220 u. |
| Hauler | Hauler | Medium | A50 P15 | 35 | 1 | 120 | 5 | 0 | 0 | 0.020 | Orbital Flight | Cargo 150; freight routes. |
| Seedship | Seedship | Medium | A160 S60 P60 | 90 | 2 | 200 | 4 | 0 | 0 | 0.040 | Colonial Charter | Founds a Keystone on an unowned rock; consumed. Takes 12 colonists from the building colony when built (requires Pop ≥ 20). Starter stock for the new colony: Alloy 80, Silica 60, Food 50, Ice 40. |
| Dart | Dart | Light | A25 P8 | 20 | 1 | 50 | 10 | 6 | 1 | 0.008 | Point Defence | Interceptor. ×1.5 damage vs Hammers. |
| Cutter | Cutter | Medium | A60 P20 | 40 | 2 | 160 | 7 | 10 | 5 | 0.020 | Gunnery | Line gunship. |
| Hammer | Hammer | Medium | A70 P25 | 45 | 2 | 120 | 6 | 2 | 22 | 0.025 | Siege Doctrine | Bomber. |
| Bulwark | Bulwark | Heavy | A180 P60 L15 | 100 | 4 | 600 | 4 | 22 | 12 | 0.050 | Capital Hulls | Heavy frigate. |

* **Berths**: faction capacity = Σ over owned rocks (4 per Keystone + 6 per active Dock Ring).
  Ships in existence + ships queued may not exceed it. Queuing a ship deducts its cost from the
  cradle's colony stock (refunded 100 % on cancel).
* A **Fleet** is a set of ships sharing a location and an order. Each faction has at most one
  **garrison** fleet per rock (idle ships in orbit). New ships join the garrison of the rock where
  they were built. Ordering a subset of a fleet's ships splits them into a new fleet.
  A fleet arriving at a friendly rock with nothing left to do merges into that rock's garrison.
* **Orders**: `Hold` (stay in orbit), `Move` (go to rock, then hold / merge), `Attack` (go to rock
  and fight, with a stance), `Survey` (Prospector: 15 s in orbit reveals all deposits to the
  faction, then returns home), `Colonize` (Seedship founds a Keystone at the requested hex or the
  best available site; the rest of the fleet becomes the garrison), `Haul` (freight route),
  `Return` (go to Home rock and merge).
* **Attack stances**: `Suppress` (defences first, then Keystone), `Raid` (industry and power
  first), `Raze` (Keystone first).
* **Movement**: a fleet in transit moves straight toward its destination rock's *current* position
  every tick at the speed of its slowest ship × `Mods.ShipSpeedMul`; it arrives when within 6 u.
  Leaving a rock owned by the fleet's faction costs Fuel = Σ ship `FuelPerU` × current distance,
  taken from that colony (launch refused if lacking). Leaving any other rock is free.
* **Freight route**: `Route{A, B, Out [NumGoods]bool, Back [NumGoods]bool, Reserve float64}`.
  At A the fleet loads the `Out` goods (split evenly across selected goods, never taking A's stock
  below `Reserve` of that good), flies to B, unloads everything B can store, loads `Back` goods
  under the same rule, returns to A, unloads, repeats. 5 s dwell at each stop. If a launch is
  refused for lack of Fuel it retries every 5 s. Both rocks must stay owned by the faction,
  otherwise the route is cleared and the fleet holds.
* **Retreat**: `RetreatAt` ∈ {0, 0.25, 0.5}: when the fleet's total HP falls below that fraction of
  its total max HP during an Attack, the order becomes `Return`.
* **Sensors / fog**: rock positions, rock owners and every surface are always visible to all.
  Deposits are hidden until revealed. Enemy fleets (in transit or in orbit) are visible only when
  inside one of your sensor radii: Keystone 260 u, Farseer Array 720 u, Prospector 220 u,
  any other ship 120 u (solar flare halves all sensor ranges).

## 8. Combat

Runs every tick at every rock where mutually hostile parties are present
(all factions are hostile to each other).

* Parties: every fleet at the rock, plus the rock owner's powered, staffed Flak Batteries and
  Lance Towers.
* Ships with `dps vs ships > 0` shoot enemy ships while any enemy ship is present, preferring armed
  ships, then unarmed. Exception: when a fleet has an `Attack` order on this rock, its ships whose
  `dps vs buildings` exceeds their `dps vs ships` (Hammers) bombard buildings from the start.
  When no enemy ships remain, every ship of an attacking fleet bombards.
* Bombardment target order by stance — Suppress: Flak/Lance/Aegis/Mass Driver, then Keystone, then
  everything else; Raid: industry and power, then defences, then Keystone; Raze: Keystone, then
  defences, then the rest. Targets are sticky until destroyed. Buildings under construction can be
  hit (they have HP proportional to progress, minimum 20 % of max).
* Turrets shoot hostile ships in orbit using the hull multipliers in the building table.
* Damage per tick = dps × 0.1 × multiplier × (0.9..1.1 random). `Mods.HPMul` scales max HP of
  ships and buildings.
* **Aegis**: damage to a building within 3 hexes of an active Aegis Projector (same owner, pool > 0)
  is taken from the pool first.
* Destroyed ship → removed (emit FX). Empty fleets are removed. Destroyed building → removed,
  Rubble (emit FX, log to owner).
* **Keystone destroyed → the colony falls**: rock owner becomes none, the colony (stock,
  colonists) is lost, the faction's remaining buildings there become **Derelict** (owner 0,
  inactive). The destroyer's faction receives a bounty of 250 Scrip. Derelicts may be demolished
  for salvage by whoever later owns the rock (refund 30 %), or **recommissioned** by the new owner
  for 40 % of the original cost (instantly active again at current HP).
* A faction with no Keystone is eliminated (`Alive = false`); its fleets are removed. When a single
  faction remains alive the game ends with that winner. If the human player is eliminated the game
  ends in defeat even if several AIs remain.
* **Mass Driver**: when it has a target rock owned by an enemy within 600 u, is powered and staffed,
  and the colony has 8 Alloy: fire a slug every 15 s. Slugs fly at 60 u/s toward the target rock's
  current position and on arrival deal 150 damage to a building chosen with the Suppress order.

## 9. Research

Per faction: research points per cycle = Σ Laboratory (6 × Eff) × `Mods.ResearchMul`. Points flow
into `Research.Current`; when cost is reached the tech is granted, its effects applied (unlocks,
`Mods` recomputed), a log entry is written and the next queued tech starts. A Lumen price is paid
when research on that tech *starts* (taken from the faction's rocks, richest first); if it cannot
be paid the tech does not start. Up to 5 techs may be queued.

| Tech (const) | Name | Tier | Points | Lumen | Requires | Grants |
|---|---|---|---|---|---|---|
| OrbitalFlight | Orbital Flight | 1 | 60 | 0 | — | Launch Cradle, Prospector, Hauler |
| VolatileCracking | Volatile Cracking | 1 | 60 | 0 | — | Cracker, Thermal Stack |
| Fabrication | Fabrication | 1 | 70 | 0 | — | Fabricator |
| CivicPlanning | Civic Planning | 1 | 50 | 0 | — | Commons |
| PointDefence | Point Defence | 1 | 90 | 0 | — | Flak Battery, Dart |
| DeepBoring | Deep Boring | 1 | 80 | 0 | — | Bore Rig output +25 % |
| CharterTrade | Charter Trade | 1 | 60 | 0 | — | Exchange Dock |
| ColonialCharter | Colonial Charter | 2 | 150 | 0 | OrbitalFlight, Fabrication | Seedship |
| Gunnery | Gunnery | 2 | 160 | 0 | PointDefence | Cutter |
| SiegeDoctrine | Siege Doctrine | 2 | 200 | 0 | Gunnery | Hammer |
| DeepScan | Deep Scan | 2 | 140 | 0 | OrbitalFlight | Farseer Array; Survey Mast range +2 |
| FieldRepair | Field Repair | 2 | 160 | 0 | Fabrication | Mender Hub |
| Photovoltaics | Tuned Photovoltaics | 2 | 150 | 0 | — | Sunvane output +30 % |
| HydroStacks | Hydroponic Stacks | 2 | 130 | 0 | CivicPlanning | Greenhouse output +35 % |
| DockEngineering | Dock Engineering | 2 | 160 | 0 | OrbitalFlight | Dock Ring |
| EfficientSmelting | Efficient Smelting | 2 | 170 | 0 | DeepBoring | Smelter output +25 % (same input) |
| MassAcceleration | Mass Acceleration | 2 | 260 | 0 | Fabrication, PointDefence | Mass Driver |
| LumenOptics | Lumen Optics | 3 | 320 | 20 | PointDefence, DeepScan | Lance Tower |
| AegisFields | Aegis Fields | 3 | 380 | 30 | LumenOptics | Aegis Projector |
| LumenFusion | Lumen Fusion | 3 | 420 | 40 | VolatileCracking, Photovoltaics | Lumen Core |
| CapitalHulls | Capital Hulls | 3 | 450 | 30 | SiegeDoctrine, DockEngineering | Bulwark |
| ReactivePlating | Reactive Plating | 3 | 350 | 0 | Gunnery, FieldRepair | Ship & building max HP +20 % |
| OverdriveThrusters | Overdrive Thrusters | 3 | 300 | 0 | DockEngineering | Ship speed +25 % |
| Automation | Automation | 3 | 400 | 0 | Fabrication, EfficientSmelting | Staff needs −25 % |

`Faction.Mods` (recomputed from techs + difficulty): `MiningMul, SolarMul, FoodMul, SmeltMul,
HPMul, ShipSpeedMul, StaffMul, ResearchMul, SurveyBonus(int), ProdMul` (ProdMul = AI difficulty
handicap applied to mining and all production: Easy 0.8, Normal 1.0, Hard 1.25 — 1.0 for humans).

## 10. Market (Exchange)

Global `Market{Pressure [NumGoods]float64}`. Price = base × (1 + Pressure). Buying costs
price × 1.12, selling pays price × 0.88. Each unit bought moves pressure +1/2000, sold −1/2000
(clamped ±0.6); pressure decays toward 0 by 1.5 % per second; small random walk each economy step.
Trading requires an active Exchange Dock at the rock, storage space (buy) / stock (sell), Scrip.
Max 200 units per command. **Sponsor settlers**: 120 Scrip → +5 colonists if housing allows,
once per cycle per rock. **Expedite**: finish a building under construction now for
`ceil(1.5 × remainingSeconds × (1 + alloyCost/100))` Scrip.

## 11. Events

Roughly one roll every 3–5 cycles (first not before cycle 4). Each affects the world via ordinary
state and is announced in the log; none may eliminate a faction on its own.

* **Micrometeor squall**: targets a random colonised rock; warning logged 20 s ahead; then 8–14
  impacts over 10 s, each 40–80 damage to the building on a random blob tile (misses hit nothing);
  Aegis absorbs.
* **Solar flare** (belt-wide, 45 s): Sunvane output ×1.6, sensor ranges ×0.5, fleets in transit
  lose 10 % of each ship's max HP in total over the flare (never below 1 HP).
* **Rich seam**: a deposit tile next to a random active Bore Rig gains 500–900 units.
* **Derelict hulk**: salvage appears in orbit of a random rock for 4 cycles
  (`Rock.Salvage`: some Alloy/Parts + Scrip). An owned rock's owner collects it automatically after
  10 s; otherwise the first faction with a fleet in orbit for 10 s collects (goods go to that
  fleet's Home colony, clipped to capacity).
* **Refugee convoy**: 4–8 colonists join a random colony with free housing.
* **Market shock**: one good's pressure jumps ±0.4.

## 12. Directives (onboarding goals, human player only)

Ordered list; the first incomplete one is "current"; completing one grants Scrip and logs a line.
1 Raise a Sunvane (30) · 2 Sink a Bore Rig beside an Ore deposit (40) · 3 Fire up a Smelter (40) ·
4 Raise a Greenhouse and an Aerator (40) · 5 Reach 20 colonists (60) · 6 Build a Laboratory and
start research (60) · 7 Research Orbital Flight (80) · 8 Build a Launch Cradle (80) ·
9 Survey another rock with a Prospector (100) · 10 Research Point Defence and raise a Flak Battery
(100) · 11 Found a second colony with a Seedship (200) · 12 Destroy a rival Keystone (400).
Each has a short title, a one-sentence hint, a reward, and a pure predicate over world state.

## 13. Game setup

`GameSettings{Seed uint64, BeltSize (Small/Medium/Large), Rivals int (1..3), Difficulty
(Easy/Normal/Hard), PlayerName string, RivalKinds []Personality (optional; random if empty),
Events bool, Directives bool}`.
Every faction starts with one Keystone colony: Pop 14, Morale 60, stock Alloy 160, Silica 130,
Ice 90, Fuel 30, Food 110, Parts 20, Ore 0, Lumen 0; Scrip 400; deposits within 3 hexes of the
Keystone revealed. Faction 1 is the human player; factions 2.. are AI.

## 14. Package architecture

```
cmd/drifthold        main: window, ebiten.RunGame(app)            (Windows GUI subsystem)
cmd/driftsim         headless runner: self-play, soak, balance stats (no Ebitengine)
internal/model       ALL shared data types, content tables, hex math, RNG, stock & query helpers,
                     command structs, log & FX event types. No game rules that need other packages.
internal/worldgen    New(settings) (*model.World, error): belt, rocks, deposits, factions, start colonies
internal/economy     Step(w): caches, staffing, power, production, mining, survey masts, life, morale,
                     population, stipend, self-repair, mender hubs
internal/construct   CanPlace/Place/Cancel/Demolish/Recommission/Expedite, SetEnabled/SetPriority,
                     Step(w): construction progress
internal/fleet       ship queue (cradles), berths, fleet split/merge, orders, movement, arrival
                     handling (survey, colonize, haul routes, salvage pickup), dock repair, sensors
internal/combat      Step(w): space & surface combat, aegis, mass drivers & slugs, colony fall,
                     elimination & victory
internal/research    Step(w), SetCurrent/Queue/Cancel, ApplyTech, RecomputeMods
internal/market      Step(w), Trade, Sponsor, price queries
internal/events      Step(w): random events
internal/directives  Step(w), definitions & progress
internal/ai          Step(w, faction): rival brains issuing commands like a player would
internal/sim         Sim{W *model.World}; NewGame(settings); Step() runs one tick in the canonical
                     order; Exec(cmd) dispatches commands to the owning system; Enqueue(cmd)
internal/save        Save/Load (gzip JSON) + metadata, settings file, save directory resolution
internal/audio       procedural synthesiser: SFX + generative ambient music (Ebitengine audio)
internal/input       per-frame input snapshot abstraction + scripted input playback
internal/gfx         theme, fonts/text, drawing primitives, procedural art (rocks, buildings, ships, icons)
internal/ui/kit      immediate-mode widget toolkit
internal/ui/...      screens & panels
internal/app         application shell
```

Dependency rule: `model` imports only the standard library. System packages import `model` only
(`ai` may additionally import `construct`, `fleet`, `research`, `market` query helpers; `sim`
imports all systems). UI packages never mutate the world — they only read it and send
`model.Command`s through the sim.

### Canonical tick order (`sim.Step`)

1. Execute `w.Pending` commands in order (errors are logged to the issuing faction as Warn), clear.
2. `fleet.Step` (movement, arrivals, routes, queues) — every tick.
3. `combat.Step` — every tick.
4. If `Tick % 10 == 0`: `construct.Step`, `economy.Step`, `research.Step`, `market.Step`,
   `events.Step`, `directives.Step`.
5. AI: for each living AI faction f, if `(Tick + 5·f) % 20 == 0`: `ai.Step(w, f)` (appends to
   `w.Pending`).
6. `w.Tick++`.

### Commands (`model.Command`, one flat struct, `Kind` discriminates)

| Kind | Fields used | Owner package | Effect |
|---|---|---|---|
| CmdBuild | Rock, Hex, BKind | construct | place building |
| CmdDemolish | Rock, Building | construct | cancel (if constructing) or demolish; salvage derelict |
| CmdRecommission | Rock, Building | construct | reactivate a derelict |
| CmdSetEnabled | Rock, Building, Flag | construct | toggle |
| CmdSetPriority | Rock, Building, Int | construct | 0..2 |
| CmdExpedite | Rock, Building | construct | pay Scrip to finish |
| CmdQueueShip | Rock, Building, SKind | fleet | add to cradle queue |
| CmdCancelShip | Rock, Building, Int | fleet | remove queue index |
| CmdFleetOrder | Fleet, Ships, Order, Target, Stance, Hex, Retreat | fleet | split if subset; set order; launch |
| CmdSetRoute | Fleet, Ships, Route | fleet | assign freight route (starts Haul) |
| CmdScuttle | Fleet, Ships | fleet | destroy own ships |
| CmdSetResearch | Tech | research | make current (replacing; progress on the old one is kept) |
| CmdQueueResearch | Tech | research | append to queue |
| CmdCancelResearch | Int | research | −1 current, else queue index |
| CmdTrade | Rock, Good, Amount (+buy / −sell) | market | trade |
| CmdSponsor | Rock | market | sponsor settlers |
| CmdSetDriverTarget | Rock, Building, Target (−1 clears) | combat | Mass Driver target |

Every command carries `Faction` (issuer). Handlers validate ownership and everything else and
return a descriptive `error` (user-readable sentence, e.g. "Not enough Alloy (need 35)").
Each owner package exports `Apply(w *model.World, c model.Command) error` handling its kinds
(returning `model.ErrNotMine` for kinds it does not own), plus typed functions for direct use.

### Log & FX

* `w.Logf(faction FactionID, sev Severity, rock RockID, format string, args...)` appends to
  `w.Log` (capped at 300 entries, oldest dropped). faction 0 = broadcast to all. Severity:
  Info, Good, Warn, Alert. `rock` (−1 none) lets the UI jump to the place.
* `w.Emit(fx model.FX)` appends a transient visual/audio cue to `w.FX` (`json:"-"`, never read by
  the sim; the app drains it after each step). FX kinds: BuildPlaced, BuildDone, BuildDestroyed,
  ShipBuilt, FleetLaunch, FleetArrive, ShotFlak, ShotLance, ShotShip, Bomb, ShipDestroyed,
  ShieldHit, SlugFired, SlugImpact, MeteorImpact, ColonyFounded, ColonyFell, TechDone,
  DirectiveDone, Alert, Trade, FlareStart, FlareEnd. Fields: Kind, Rock, Hex, Faction, Fleet,
  A/B (Vec2 belt positions, when relevant), Amount.
