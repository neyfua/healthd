'use strict';

const apptsBody = document.getElementById('appts');
const formMsg = document.getElementById('form-msg');
const metricsBox = document.getElementById('metrics');

async function getJSON(path) {
  const res = await fetch(path, { headers: { Accept: 'application/json' } });
  if (!res.ok) throw new Error(path + ' -> ' + res.status);
  return res.json();
}

async function postJSON(path, body) {
  const res = await fetch(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  const text = await res.text();
  if (!res.ok) throw new Error(text || res.statusText);
  return JSON.parse(text);
}

async function cancel(id) {
  const res = await fetch('/api/v1/appointments/' + encodeURIComponent(id), { method: 'DELETE' });
  if (!res.ok) {
    formMsg.textContent = 'Cancel failed: ' + res.status;
    return;
  }
  formMsg.textContent = '';
  loadAppointments();
}

function renderAppointments(list) {
  apptsBody.textContent = '';
  for (const a of list) {
    const tr = document.createElement('tr');
    if (a.status === 'cancelled') tr.className = 'cancelled';

    const cells = [a.id, a.patient, a.reason, new Date(a.time).toLocaleString(), a.status];
    for (const value of cells) {
      const td = document.createElement('td');
      td.textContent = value;
      tr.appendChild(td);
    }

    const actions = document.createElement('td');
    if (a.status !== 'cancelled') {
      const btn = document.createElement('button');
      btn.textContent = 'Cancel';
      btn.addEventListener('click', () => cancel(a.id));
      actions.appendChild(btn);
    }
    tr.appendChild(actions);
    apptsBody.appendChild(tr);
  }
}

async function loadAppointments() {
  try {
    renderAppointments(await getJSON('/api/v1/appointments'));
  } catch (err) {
    formMsg.textContent = 'Load failed: ' + err.message;
  }
}

async function loadMetrics() {
  try {
    const res = await fetch('/metrics');
    metricsBox.textContent = await res.text();
  } catch (err) {
    metricsBox.textContent = 'Metrics unavailable: ' + err.message;
  }
}

async function setBadge(el, path) {
  try {
    const res = await fetch(path);
    el.textContent = path === '/livez' ? 'LIVE' : 'READY';
    el.className = 'badge ' + (res.ok ? 'ok' : 'bad');
  } catch (err) {
    el.className = 'badge bad';
  }
}

document.getElementById('create-form').addEventListener('submit', async (event) => {
  event.preventDefault();
  const when = document.getElementById('when').value;
  try {
    await postJSON('/api/v1/appointments', {
      patient: document.getElementById('patient').value,
      reason: document.getElementById('reason').value,
      time: new Date(when).toISOString(),
    });
    formMsg.textContent = '';
    document.getElementById('patient').value = '';
    document.getElementById('reason').value = '';
    document.getElementById('when').value = '';
    loadAppointments();
  } catch (err) {
    formMsg.textContent = 'Booking failed: ' + err.message;
  }
});

setBadge(document.getElementById('live-badge'), '/livez');
setBadge(document.getElementById('ready-badge'), '/readyz');
loadAppointments();
loadMetrics();
setInterval(loadAppointments, 15000);
