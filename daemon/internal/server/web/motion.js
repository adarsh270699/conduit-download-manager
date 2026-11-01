// Conduit motion: dependency-free springs for the dashboard, following Apple's fluid
// interface rules. Every move is a critically damped spring (no bounce) set by its
// response time, settles at a visible precision, and carries its velocity into any
// retarget, so an interrupted move continues instead of stopping and starting again.
// Reduced motion keeps opacity and state changes and drops movement.
(function (global) {
  'use strict';

  // Response in seconds: the time for a critically damped move to reach about 99%.
  // Apple's default range is 0.3 to 0.4 seconds.
  const RESPONSE = { move: 0.4, enter: 0.3 };
  const SUBSTEP = 1 / 240;   // small steps keep the stiffer springs stable
  const MAX_FRAME = 1 / 30;  // a long frame (a background tab) advances a spring by at most this
  const SETTLE = 0.005;      // settle within 0.5% of the distance travelled
  const MIN_TOLERANCE = 0.1; // and never tighter than a tenth of a pixel

  const active = new Set();
  let frame = 0;
  let last = 0;

  function reduced() {
    return window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  }

  // A critically damped spring with the given response: omega = 2π / response,
  // stiffness = omega², damping = 2 × omega.
  function constants(response) {
    const omega = (Math.PI * 2) / response;
    return { stiffness: omega * omega, damping: 2 * omega };
  }

  // Advance one channel by dt seconds, in substeps.
  function integrate(ch, dt, k) {
    let t = dt;
    while (t > 0) {
      const h = Math.min(SUBSTEP, t);
      ch.v += (k.stiffness * (ch.to - ch.x) - k.damping * ch.v) * h;
      ch.x += ch.v * h;
      t -= h;
    }
  }

  // At rest when the value is within its tolerance and moving slower than a few pixels a
  // second, which is too slow to see.
  function atRest(ch) {
    return Math.abs(ch.to - ch.x) < ch.tol && Math.abs(ch.v) < ch.tol * 4;
  }

  function values(state) {
    const out = {};
    for (const name in state.channels) out[name] = state.channels[name].x;
    return out;
  }

  function settle(el) {
    const s = el._spring;
    for (const name in s.channels) s.channels[name].x = s.channels[name].to;
    active.delete(el);
    delete el._spring;
    if (s.end) s.end();
    else s.write(values(s));
  }

  function tick(now) {
    const dt = Math.min((now - last) / 1000, MAX_FRAME);
    last = now;
    for (const el of Array.from(active)) {
      const s = el._spring;
      // A delayed start holds its first frame still until its delay has passed.
      if (s.delay > 0) {
        s.delay -= dt;
        continue;
      }
      let rest = true;
      for (const name in s.channels) {
        const ch = s.channels[name];
        integrate(ch, dt, s.k);
        if (!atRest(ch)) rest = false;
      }
      if (rest) settle(el);
      else s.write(values(s));
    }
    frame = active.size ? requestAnimationFrame(tick) : 0;
  }

  // Start or retarget the springs on an element. A channel keeps its position and velocity
  // unless a start value is given, so a retarget carries the motion already in progress.
  function drive(el, targets, options) {
    const k = constants(options.response);
    let s = el._spring;
    if (!s) s = el._spring = { channels: {} };
    s.k = k;
    s.write = options.write;
    s.end = options.end || null;
    s.delay = options.delay || 0;

    for (const name in targets) {
      const t = targets[name];
      let ch = s.channels[name];
      if (!ch) ch = s.channels[name] = { x: t.to, v: 0, to: t.to, tol: MIN_TOLERANCE };
      if (t.from !== undefined) ch.x = t.from;
      ch.to = t.to;
      ch.tol = Math.max(t.min === undefined ? MIN_TOLERANCE : t.min, SETTLE * Math.abs(t.to - ch.x));
    }

    active.add(el);
    if (!frame) {
      last = performance.now();
      frame = requestAnimationFrame(tick);
    }
    s.write(values(s));
  }

  // Stop a move in progress and return the element to its layout position.
  function cancel(el) {
    const s = el && el._spring;
    if (!s) return;
    active.delete(el);
    delete el._spring;
    if (s.end) s.end();
  }

  // The vertical offset a row is still travelling through, so a reorder can continue from it.
  function offsetY(el) {
    return el._spring && el._spring.channels.y ? el._spring.channels.y.x : 0;
  }

  // Rise into place and fade in, after an optional delay in seconds. Reduced motion fades without moving.
  function enter(el, distance, delay) {
    if (!el) return;
    const fresh = !el._spring;
    const quiet = reduced();
    const targets = { opacity: { to: 1, from: fresh ? 0 : undefined, min: 0.002 } };
    if (!quiet) targets.y = { to: 0, from: fresh ? distance : undefined };
    drive(el, targets, {
      response: RESPONSE.enter,
      delay: delay,
      write: (v) => {
        if (v.y !== undefined) el.style.transform = 'translateY(' + v.y + 'px)';
        el.style.opacity = String(v.opacity);
      },
      end: () => {
        el.style.transform = '';
        el.style.opacity = '';
      },
    });
  }

  // Glide a row from startY, its old position relative to its new layout place, down to rest.
  function flip(el, startY) {
    if (!el) return;
    if (reduced()) {
      cancel(el);
      return;
    }
    if (!el._spring && Math.abs(startY) < 0.5) return;
    drive(el, { y: { to: 0, from: startY } }, {
      response: RESPONSE.move,
      write: (v) => {
        el.style.transform = 'translateY(' + v.y + 'px)';
      },
      end: () => {
        el.style.transform = '';
      },
    });
  }

  // Place an element at a position and width at once. Used for the first placement.
  function place(el, x, w) {
    if (!el) return;
    cancel(el);
    el._place = { x: x, w: w };
    el.style.transform = 'translateX(' + x + 'px) scaleX(' + w + ')';
  }

  // Slide a thin element, such as the tab underline, to a new x and width.
  function slideTo(el, x, w) {
    if (!el) return;
    if (reduced()) {
      place(el, x, w);
      return;
    }
    const from = el._place || { x: x, w: w };
    const fresh = !el._spring;
    drive(el, {
      x: { to: x, from: fresh ? from.x : undefined },
      w: { to: w, from: fresh ? from.w : undefined },
    }, {
      response: RESPONSE.move,
      write: (v) => {
        el._place = { x: v.x, w: v.w };
        el.style.transform = 'translateX(' + v.x + 'px) scaleX(' + v.w + ')';
      },
      end: null,
    });
  }

  // Ease an element's height from its old value to the value its new content needs, so a change of
  // state reshapes the box instead of jumping. An interrupted resize keeps its position and velocity.
  function resize(el, fromHeight) {
    if (!el) return;
    if (reduced()) {
      cancel(el);
      return;
    }
    const inFlight = !!(el._spring && el._spring.channels.h);
    // Measure the natural height with the inline height cleared. drive() sets the height again in this task.
    el.style.height = '';
    const to = el.getBoundingClientRect().height;
    if (!inFlight && Math.abs(to - fromHeight) < 0.5) return;
    drive(el, { h: { to: to, from: inFlight ? undefined : fromHeight, min: 0.5 } }, {
      response: RESPONSE.move,
      write: (v) => {
        el.style.height = v.h + 'px';
      },
      end: () => {
        el.style.height = '';
      },
    });
  }

  // Hide a layer after its exit animation finishes. Reduced motion uses the fade only.
  // An optional callback runs once the layer is hidden, so the next layer can take its place.
  function leave(el, then) {
    if (!el || el.classList.contains('hidden') || el.classList.contains('is-leaving')) return;
    el.classList.add('is-leaving');
    const finish = (event) => {
      if (event && event.target !== el) return;
      cancelLeave(el);
      el.classList.add('hidden');
      el.classList.remove('is-leaving');
      if (then) then();
    };
    el._leaveFinish = finish;
    el.addEventListener('animationend', finish);
    // Safety net for a page where the exit animation never runs.
    el._leaveTimer = setTimeout(finish, 260);
  }

  // Abandon a pending exit, for example when the dialog opens again.
  function cancelLeave(el) {
    if (!el) return;
    if (el._leaveFinish) el.removeEventListener('animationend', el._leaveFinish);
    if (el._leaveTimer) clearTimeout(el._leaveTimer);
    el._leaveFinish = null;
    el._leaveTimer = null;
    el.classList.remove('is-leaving');
  }

  // Ease a box's height to zero, then run done. A box reopened mid-way keeps its velocity, and the old done is dropped.
  function collapse(el, done) {
    if (!el) return;
    if (reduced()) {
      cancel(el);
      el.style.height = '';
      done();
      return;
    }
    const inFlight = !!(el._spring && el._spring.channels.h);
    const from = el.getBoundingClientRect().height;
    drive(el, { h: { to: 0, from: inFlight ? undefined : from, min: 0.5 } }, {
      response: RESPONSE.move,
      write: (v) => {
        el.style.height = v.h + 'px';
      },
      end: () => {
        el.style.height = '';
        done();
      },
    });
  }

  // Remove a toast after its exit animation finishes.
  function dismiss(el) {
    if (!el || el._dismissed) return;
    el._dismissed = true;
    el.classList.add('is-leaving');
    const finish = (event) => {
      if (event && event.target !== el) return;
      clearTimeout(el._leaveTimer);
      el.remove();
    };
    el.addEventListener('animationend', finish);
    el._leaveTimer = setTimeout(finish, 200);
  }

  global.ConduitMotion = {
    reduced: reduced,
    enter: enter,
    flip: flip,
    offsetY: offsetY,
    place: place,
    slideTo: slideTo,
    resize: resize,
    collapse: collapse,
    cancel: cancel,
    leave: leave,
    cancelLeave: cancelLeave,
    dismiss: dismiss,
  };
})(window);
