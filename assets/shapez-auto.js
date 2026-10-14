const METADATA = {
    website: "https://github.com/Songxwn/shapez-auto",
    author: "shapez-auto",
    name: "shapez-auto",
    version: "0.1.6",
    id: "shapez-auto",
    description: "Connects to a local planner and completes levels automatically.",
    minimumGameVersion: ">=1.5.0",
    doesNotAffectSavegame: true,
};

class Mod extends shapez.Mod {
    init() {
        this.root = null;
        this.lastPlan = "";
        this.cachedLevel = 0;
        this.cachedResources = [];
        this.busy = false;
        this.playing = false;
        this.missed = 0;
        this.timer = 0;
        const start = (root) => {
            this.root = root;
            this.lastPlan = "";
            this.cachedLevel = 0;
            this.badge("shapez-auto 已连接本局");
            this.ensureLoop();
        };
        this.signals.gameStarted.add(start);
        this.signals.gameInitialized.add(start);
    }

    ensureLoop() {
        if (this.timer) return;
        this.timer = setInterval(() => this.tick(), 1200);
    }

    badge(text) {
        let el = document.getElementById("shapez-auto-badge");
        if (!el) {
            el = document.createElement("div");
            el.id = "shapez-auto-badge";
            el.style.cssText = "position:fixed;left:8px;bottom:8px;z-index:99999;background:#111;color:#b6f3b6;padding:6px 8px;font:12px/1.4 sans-serif;pointer-events:none;border-radius:6px";
            document.body.appendChild(el);
        }
        el.textContent = "shapez-auto: " + text;
    }

    async tick() {
        if (!this.root || this.busy || !this.root.hubGoals) return;
        this.busy = true;
        try {
            const levelNow = this.root.hubGoals.level;
            const goalNow = this.root.hubGoals.currentGoal;
            const shapeNow = goalNow && goalNow.definition ? goalNow.definition.getHash() : "";
            const levelKey = levelNow + ":" + shapeNow;
            if (this.playing && shapeNow && this.clearedKey !== levelKey) {
                this.clearFactory();
                this.lastPlan = "";
                this.missed = 0;
                this.clearedKey = levelKey;
                this.badge("第 " + levelNow + " 关，已清空流水线，等待外部模型重新设计");
            }
            if (this.playing && this.lastPlan && this.root.hubGoals.getCurrentGoalDelivered() <= 0) {
                this.fixDelivery();
            }
            const snap = this.snapshot();
            if (this.playing) this.dismissOverlays();
            const res = await fetch("http://127.0.0.1:17831/api/tick", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify(snap),
            });
            if (!res.ok) throw new Error("控制面 " + res.status);
            const plan = await res.json();
            this.playing = plan.action === "build";
            if (this.playing) this.dismissOverlays();
            if (plan.action === "build" && plan.id && plan.id !== this.lastPlan) {
                this.apply(plan);
                this.lastPlan = plan.id;
                this.badge("第 " + plan.level + " 关已摆厂，加速 x" + (plan.warp || 1));
            } else if (plan.action === "error") {
                this.badge(plan.error || "规划失败");
            } else if (plan.action === "done") {
                this.setWarp(1);
                this.badge(plan.comment || "已完成");
            } else if (plan.action === "idle") {
                this.badge("等待控制面点击开始");
            } else if (plan.action === "build") {
                this.setWarp(plan.warp || 8);
                this.buyUpgrades();
            }
        } catch (err) {
            this.badge(String(err && err.message ? err.message : err));
        } finally {
            this.busy = false;
        }
    }

    snapshot() {
        const root = this.root;
        const goal = root.hubGoals.currentGoal;
        const level = root.hubGoals.level;
        if (this.cachedLevel !== level) {
            this.cachedResources = this.scanResources();
            this.cachedLevel = level;
        }
        const names = [
            "reward_cutter_and_trash", "reward_rotater", "reward_painter", "reward_mixer",
            "reward_stacker", "reward_balancer", "reward_tunnel", "reward_rotater_ccw",
            "reward_rotater_180", "reward_miner_chainable", "reward_underground_belt_tier_2",
            "reward_belt_reader", "reward_splitter", "reward_cutter_quad", "reward_painter_double",
            "reward_storage", "reward_merger", "reward_wires_painter_and_levers", "reward_filter",
            "reward_freeplay",
        ];
        const rewards = [];
        for (let i = 0; i < names.length; i++) {
            try {
                if (root.hubGoals.isRewardUnlocked(names[i])) rewards.push(names[i]);
            } catch (err) {}
        }
        return {
            level: level,
            shape: goal && goal.definition ? goal.definition.getHash() : "",
            required: goal ? goal.required : 0,
            delivered: root.hubGoals.getCurrentGoalDelivered(),
            throughputOnly: !!(goal && goal.throughputOnly),
            rewards: rewards,
            resources: this.cachedResources,
            demoEnded: root.hubGoals.isEndOfDemoReached(),
            freeplay: root.hubGoals.isFreePlay(),
            missed: this.missed || 0,
        };
    }

    scanResources() {
        const root = this.root;
        const map = root.map;
        const chunkSize = (shapez.globalConfig && shapez.globalConfig.mapChunkSize) || 16;
        const radius = 4;
        const found = [];
        for (let cy = -radius; cy <= radius; cy++) {
            for (let cx = -radius; cx <= radius; cx++) {
                if (typeof map.getOrCreateChunk === "function") map.getOrCreateChunk(cx, cy);
                else if (typeof map.getChunk === "function") map.getChunk(cx, cy, true);
            }
        }
        for (let y = -radius * chunkSize; y < radius * chunkSize; y++) {
            for (let x = -radius * chunkSize; x < radius * chunkSize; x++) {
                let item = null;
                try { item = map.getLowerLayerContentXY(x, y); } catch (err) { item = null; }
                if (!item) continue;
                const kind = item.getItemType ? item.getItemType() : "";
                if ((kind === "shape" || item.definition) && item.definition && item.definition.getHash) {
                    found.push({ x: x, y: y, kind: "shape", key: item.definition.getHash(), d: Math.abs(x) + Math.abs(y) });
                } else if (kind === "color" || item.color) {
                    found.push({ x: x, y: y, kind: "color", key: item.color, d: Math.abs(x) + Math.abs(y) });
                }
            }
        }
        const kept = {};
        found.sort((a, b) => a.d - b.d);
        const out = [];
        for (let i = 0; i < found.length; i++) {
            const key = found[i].kind + ":" + found[i].key;
            kept[key] = kept[key] || 0;
            const nearHub = Math.abs(found[i].x) <= 8 && Math.abs(found[i].y) <= 8;
            if (kept[key] >= 24 && !nearHub) continue;
            kept[key]++;
            out.push({ x: found[i].x, y: found[i].y, kind: found[i].kind, key: found[i].key });
        }
        return out;
    }

    fixDelivery() {
        const root = this.root;
        if (!root || !root.logic) return;
        const goal = root.hubGoals.currentGoal;
        const shape = goal && goal.definition ? goal.definition.getHash() : "";
        let changed = false;
        const run = () => {
            if (this.swapWrongOutput(root, shape)) changed = true;
            if (this.routeOutputsToHub(root)) changed = true;
        };
        try {
            root.logic.performBulkOperation(run);
        } catch (err) {
            root.bulkOperationRunning = false;
        }
        if (changed) {
            this.missed = 0;
            this.badge("成品没有进入中枢，已改接到入口");
        }
    }

    dirs() {
        return {
            order: ["top", "right", "bottom", "left"],
            vec: { top: [0, -1], right: [1, 0], bottom: [0, 1], left: [-1, 0] },
            opp: { top: "bottom", bottom: "top", left: "right", right: "left" },
        };
    }

    turnDir(dir, angle) {
        const order = this.dirs().order;
        const i = order.indexOf(dir);
        if (i < 0) return "";
        return order[(i + angle / 90) % 4];
    }

    beltPose(inDir, outDir) {
        if (!inDir || !outDir || inDir === outDir) return null;
        const rotForIn = { bottom: 0, left: 90, top: 180, right: 270 };
        const rot = rotForIn[inDir];
        if (rot === undefined) return null;
        const locals = ["top", "left", "right"];
        for (let variant = 0; variant < locals.length; variant++) {
            if (this.turnDir(locals[variant], rot) === outDir) {
                return { rotation: rot, rotationVariant: variant };
            }
        }
        return null;
    }

    entityAt(root, x, y) {
        try { return root.map.getLayerContentXY(x, y, "regular"); } catch (err) { return null; }
    }

    tileBlocked(root, x, y, replace) {
        if (x < -80 || x > 160 || y < -80 || y > 220) return true;
        const ent = this.entityAt(root, x, y);
        if (ent) return !(replace && replace[ent.uid]);
        try { return !!root.map.getLowerLayerContentXY(x, y); } catch (err) { return false; }
    }

    findHub(root) {
        const ents = root.entityMgr.entities || [];
        for (let i = 0; i < ents.length; i++) {
            const e = ents[i];
            if (e && !e.destroyed && e.components && e.components.Hub) return e;
        }
        return null;
    }

    hubFeedTiles(hub) {
        const d = this.dirs();
        const st = hub.components.StaticMapEntity;
        const slots = (hub.components.ItemAcceptor && hub.components.ItemAcceptor.slots) || [];
        const out = [];
        const seen = {};
        for (let i = 0; i < slots.length; i++) {
            const slot = slots[i];
            const world = st.localTileToWorld(slot.pos);
            const fromWorld = st.localDirectionToWorld(slot.direction);
            const v = d.vec[fromWorld];
            if (!v) continue;
            const x = world.x + v[0];
            const y = world.y + v[1];
            const key = x + "," + y;
            if (seen[key]) continue;
            seen[key] = true;
            out.push({ x: x, y: y, out: d.opp[fromWorld] });
        }
        return out;
    }

    placeBuilding(root, id, x, y, rotation, rotationVariant) {
        const registry = shapez.gMetaBuildingRegistry;
        if (!registry.hasId(id)) return null;
        try {
            return root.logic.tryPlaceBuilding({
                origin: new shapez.Vector(x, y),
                rotation: rotation || 0,
                originalRotation: rotation || 0,
                rotationVariant: rotationVariant || 0,
                variant: "default",
                building: registry.findById(id),
            });
        } catch (err) {
            return null;
        }
    }

    isTrash(ent) {
        return !!(ent && ent.components && ent.components.ItemProcessor && ent.components.ItemProcessor.type === "trash");
    }

    isMachine(ent) {
        return !!(ent && ent.components && ent.components.ItemEjector && !ent.components.Belt && !ent.components.Hub);
    }

    followBelts(root, start) {
        const belt = root.systemMgr.systems.belt;
        const chain = [start];
        const seen = {};
        seen[start.uid] = true;
        let cur = start;
        for (let n = 0; n < 400; n++) {
            let next = null;
            try { next = belt.findFollowUpEntity(cur); } catch (err) { next = null; }
            if (!next || seen[next.uid]) break;
            seen[next.uid] = true;
            chain.push(next);
            cur = next;
        }
        return chain;
    }

    processorBelts(root) {
        const ents = root.entityMgr.entities || [];
        const out = [];
        for (let i = 0; i < ents.length; i++) {
            const e = ents[i];
            if (!e || e.destroyed || !e.components || !e.components.Belt) continue;
            if (this.feedingMachine(root, e)) out.push(e);
        }
        return out;
    }

    feedingMachine(root, beltEnt) {
        const origin = beltEnt.components.StaticMapEntity.origin;
        const ents = root.entityMgr.entities || [];
        for (let i = 0; i < ents.length; i++) {
            const machine = ents[i];
            if (!this.isMachine(machine) || machine.destroyed) continue;
            const landings = this.ejectLandings(machine);
            for (let k = 0; k < landings.length; k++) {
                if (landings[k].x === origin.x && landings[k].y === origin.y) return machine;
            }
        }
        return null;
    }

    beltShape(ent) {
        const path = ent.components.Belt.assignedPath;
        const items = path && path.items;
        if (!items) return "";
        for (let i = 0; i < items.length; i++) {
            const item = items[i][1];
            if (item && item.definition && item.definition.getHash) return item.definition.getHash();
        }
        return "";
    }

    ejectLandings(entity) {
        const d = this.dirs();
        const st = entity.components.StaticMapEntity;
        const ej = entity.components.ItemEjector;
        if (!ej) return [];
        const out = [];
        for (let i = 0; i < ej.slots.length; i++) {
            const slot = ej.slots[i];
            const origin = st.localTileToWorld(slot.pos);
            const travel = st.localDirectionToWorld(slot.direction);
            const v = d.vec[travel];
            if (!v) continue;
            out.push({ x: origin.x + v[0], y: origin.y + v[1], travel: travel, from: d.opp[travel] });
        }
        return out;
    }

    swapWrongOutput(root, goal) {
        if (!goal) return false;
        const starters = this.processorBelts(root);
        let swapped = false;
        for (let i = 0; i < starters.length; i++) {
            const beltEnt = starters[i];
            const hash = this.beltShape(beltEnt);
            if (!hash || hash === goal) continue;
            const proc = this.feedingMachine(root, beltEnt);
            if (!proc || proc.components.Miner) continue;
            const landings = this.ejectLandings(proc);
            const origin = beltEnt.components.StaticMapEntity.origin;
            let trash = null;
            let current = null;
            for (let k = 0; k < landings.length; k++) {
                const tile = landings[k];
                if (tile.x === origin.x && tile.y === origin.y) current = tile;
                const ent = this.entityAt(root, tile.x, tile.y);
                if (this.isTrash(ent)) trash = { tile: tile, ent: ent };
            }
            if (!current || !trash) continue;
            try { root.logic.tryDeleteBuilding(beltEnt); } catch (err) {}
            try { root.logic.tryDeleteBuilding(trash.ent); } catch (err) {}
            const pose = this.beltPose(trash.tile.from, trash.tile.travel);
            if (!pose) continue;
            this.placeBuilding(root, "trash", current.x, current.y, 0, 0);
            this.placeBuilding(root, "belt", trash.tile.x, trash.tile.y, pose.rotation, pose.rotationVariant);
            swapped = true;
        }
        return swapped;
    }

    routeOutputsToHub(root) {
        const hub = this.findHub(root);
        if (!hub) return false;
        const feeds = this.hubFeedTiles(hub);
        if (!feeds.length) return false;
        const starters = this.processorBelts(root);
        let changed = false;
        for (let i = 0; i < starters.length; i++) {
            const start = starters[i];
            if (!start || start.destroyed || !start.components || !start.components.Belt) continue;
            const chain = this.followBelts(root, start);
            const last = chain[chain.length - 1];
            const front = this.frontOf(root, last);
            if (front.ent && front.ent.components && front.ent.components.Hub) continue;
            if (this.isMachine(front.ent) && !this.isTrash(front.ent)) continue;
            const straight = this.outputStep(root, start);
            if (!straight) continue;
            const replace = {};
            for (let k = 1; k < chain.length; k++) replace[chain[k].uid] = true;
            if (this.bfsToFeed(root, straight.x, straight.y, straight.travel, feeds, replace)) {
                this.straightenOutput(root, start, straight);
                changed = true;
            }
        }
        return changed;
    }

    frontOf(root, ent) {
        const d = this.dirs();
        const st = ent.components.StaticMapEntity;
        const dir = st.localDirectionToWorld(ent.components.Belt.direction);
        const v = d.vec[dir] || [0, 0];
        const x = st.origin.x + v[0];
        const y = st.origin.y + v[1];
        return { x: x, y: y, dir: dir, ent: this.entityAt(root, x, y) };
    }

    outputStep(root, beltEnt) {
        const d = this.dirs();
        const origin = beltEnt.components.StaticMapEntity.origin;
        for (let i = 0; i < d.order.length; i++) {
            const from = d.order[i];
            const v = d.vec[from];
            const src = this.entityAt(root, origin.x + v[0], origin.y + v[1]);
            if (!this.isMachine(src)) continue;
            const travel = d.opp[from];
            const pose = this.beltPose(from, travel);
            if (!pose) return null;
            const ev = d.vec[travel];
            return { x: origin.x + ev[0], y: origin.y + ev[1], travel: travel, from: from, pose: pose, belt: beltEnt };
        }
        return null;
    }

    straightenOutput(root, beltEnt, straight) {
        const localOut = ["top", "left", "right"][straight.pose.rotationVariant];
        const st = beltEnt.components.StaticMapEntity;
        if (st.rotation === straight.pose.rotation && beltEnt.components.Belt.direction === localOut) return;
        const origin = st.origin;
        try { root.logic.tryDeleteBuilding(beltEnt); } catch (err) {}
        this.placeBuilding(root, "belt", origin.x, origin.y, straight.pose.rotation, straight.pose.rotationVariant);
    }

    bfsToFeed(root, x, y, travel, feeds, replace) {
        const d = this.dirs();
        const inDir = d.opp[travel];
        if (this.tileBlocked(root, x, y, replace)) return false;
        const feedAt = {};
        for (let i = 0; i < feeds.length; i++) feedAt[feeds[i].x + "," + feeds[i].y] = feeds[i];
        const q = [{ x: x, y: y, in: inDir, prev: -1 }];
        const seen = {};
        seen[x + "," + y + "," + inDir] = true;
        for (let qi = 0; qi < q.length && qi < 4000; qi++) {
            const cur = q[qi];
            const feed = feedAt[cur.x + "," + cur.y];
            if (feed && this.beltPose(cur.in, feed.out)) {
                return this.stampPath(root, q, qi, feed.out);
            }
            for (let k = 0; k < d.order.length; k++) {
                const out = d.order[k];
                if (!this.beltPose(cur.in, out)) continue;
                const v = d.vec[out];
                const nx = cur.x + v[0];
                const ny = cur.y + v[1];
                const nk = nx + "," + ny + "," + d.opp[out];
                if (seen[nk]) continue;
                if (this.tileBlocked(root, nx, ny, replace)) continue;
                seen[nk] = true;
                q.push({ x: nx, y: ny, in: d.opp[out], prev: qi });
            }
        }
        return false;
    }

    stampPath(root, q, at, goalOut) {
        const belts = [];
        let out = goalOut;
        const d = this.dirs();
        while (at >= 0) {
            const cur = q[at];
            const pose = this.beltPose(cur.in, out);
            if (!pose) return false;
            belts.push({ x: cur.x, y: cur.y, rotation: pose.rotation, rotationVariant: pose.rotationVariant });
            out = d.opp[cur.in];
            at = cur.prev;
        }
        for (let i = belts.length - 1; i >= 0; i--) {
            const belt = belts[i];
            if (!this.placeBuilding(root, "belt", belt.x, belt.y, belt.rotation, belt.rotationVariant)) return false;
        }
        return true;
    }

    clearFactory() {
        const root = this.root;
        if (!root || !root.logic || !root.entityMgr) return;
        const run = () => {
            const list = [];
            const ents = root.entityMgr.entities || [];
            for (let i = 0; i < ents.length; i++) list.push(ents[i]);
            for (let i = 0; i < list.length; i++) {
                const entity = list[i];
                if (!entity || !entity.components || entity.components.Hub || entity.destroyed) continue;
                try { root.logic.tryDeleteBuilding(entity); } catch (err) {}
            }
        };
        try {
            root.logic.performBulkOperation(run);
        } catch (err) {
            root.bulkOperationRunning = false;
        }
    }

    apply(plan) {
        const root = this.root;
        this.setWarp(plan.warp || 8);
        const run = () => {
            const list = [];
            const ents = root.entityMgr.entities || [];
            for (let i = 0; i < ents.length; i++) list.push(ents[i]);
            for (let i = 0; i < list.length; i++) {
                const entity = list[i];
                if (!entity || !entity.components || entity.components.Hub || entity.destroyed) continue;
                try { root.logic.tryDeleteBuilding(entity); } catch (err) {}
            }
            const registry = shapez.gMetaBuildingRegistry;
            const places = plan.placements || [];
            let missed = 0;
            for (let i = 0; i < places.length; i++) {
                const place = places[i];
                if (!registry.hasId(place.id)) {
                    missed++;
                    continue;
                }
                const building = registry.findById(place.id);
                const rotation = place.rotation || 0;
                const rotationVariant = place.rotationVariant || 0;
                let entity = null;
                try {
                    entity = root.logic.tryPlaceBuilding({
                        origin: new shapez.Vector(place.x, place.y),
                        rotation: rotation,
                        originalRotation: rotation,
                        rotationVariant: rotationVariant,
                        variant: place.variant || "default",
                        building: building,
                    });
                } catch (err) {
                    entity = null;
                }
                if (!entity) missed++;
            }
            if (this.routeOutputsToHub(root)) missed = 0;
            this.missed = missed;
        };
        try {
            root.logic.performBulkOperation(run);
        } catch (err) {
            root.bulkOperationRunning = false;
            this.badge("摆放失败: " + err);
        }
        this.buyUpgrades();
    }

    dismissOverlays() {
        const root = this.root;
        const hud = root && root.hud;
        if (hud && hud.parts) {
            const unlock = hud.parts.unlockNotification;
            if (unlock && unlock.visible) {
                try {
                    if (unlock.element) {
                        const btn = unlock.element.querySelector("button.close");
                        if (btn) btn.classList.add("unlocked");
                    }
                    if (typeof unlock.close === "function") unlock.close();
                    if (hud.signals && hud.signals.unlockNotificationFinished && hud.signals.unlockNotificationFinished.dispatch) {
                        hud.signals.unlockNotificationFinished.dispatch();
                    }
                } catch (err) {}
                if (unlock.visible) this.clickNextButtons();
            }
            const dialogs = hud.parts.dialogs;
            if (dialogs && dialogs.dialogStack && dialogs.dialogStack.length) {
                const top = dialogs.dialogStack[dialogs.dialogStack.length - 1];
                try {
                    const buttons = top.element ? top.element.querySelectorAll("button") : [];
                    let clicked = false;
                    for (let i = buttons.length - 1; i >= 0; i--) {
                        buttons[i].click();
                        clicked = true;
                        break;
                    }
                    if (!clicked && typeof dialogs.closeDialog === "function") dialogs.closeDialog(top);
                } catch (err) {}
            }
            const advantages = hud.parts.standaloneAdvantages;
            if (advantages && advantages.visible && typeof advantages.close === "function") {
                try { advantages.close(); } catch (err) {}
            }
        } else {
            this.clickNextButtons();
        }
    }

    clickNextButtons() {
        const nodes = document.querySelectorAll("#ingame_HUD_UnlockNotification button.close, #ingame_HUD_ModalDialogs button");
        for (let i = 0; i < nodes.length; i++) {
            const el = nodes[i];
            const text = (el.innerText || "");
            const next = el.classList.contains("close") || text.indexOf("下一") >= 0 || text.toLowerCase().indexOf("next") >= 0 || text === "OK" || text === "Ok";
            if (!next) continue;
            el.classList.add("unlocked");
            try { el.click(); } catch (err) {}
        }
    }

    buyUpgrades() {
        const goals = this.root.hubGoals;
        const ids = ["belt", "miner", "processors", "painting"];
        for (let n = 0; n < 8; n++) {
            let bought = false;
            for (let i = 0; i < ids.length; i++) {
                try {
                    if (goals.canUnlockUpgrade(ids[i]) && goals.tryUnlockUpgrade(ids[i])) bought = true;
                } catch (err) {}
            }
            if (!bought) break;
        }
    }

    setWarp(mult) {
        const speed = this.root.time && (this.root.time.speed || this.root.time.getSpeed());
        if (!speed) return;
        if (!speed.__shapezAuto) {
            const baseMul = speed.getTimeMultiplier.bind(speed);
            const baseMax = speed.getMaxLogicStepsInQueue.bind(speed);
            speed.getTimeMultiplier = () => baseMul() * (speed.__warp || 1);
            speed.getMaxLogicStepsInQueue = () => Math.max(baseMax(), speed.__warp || 1);
            speed.__shapezAuto = true;
        }
        speed.__warp = Math.max(1, mult | 0);
    }
}
