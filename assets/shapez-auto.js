const METADATA = {
    website: "https://github.com/Songxwn/shapez-auto",
    author: "shapez-auto",
    name: "shapez-auto",
    version: "0.1.4",
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
            if (kept[key] >= 24) continue;
            kept[key]++;
            out.push({ x: found[i].x, y: found[i].y, kind: found[i].kind, key: found[i].key });
        }
        return out;
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
