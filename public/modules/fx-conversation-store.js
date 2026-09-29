const databaseName = 'miao-fx-private-conversations';
const databaseVersion = 1;
const storeName = 'conversations';
const maxCheckpointBytes = 4 * 1024 * 1024;
const maxTranscriptCharacters = 256 * 1024;
const maxTranscriptMessages = 240;

let databasePromise;

export function fxAuthorizationScope(tenant, apps) {
  const permissions = (apps || [])
    .map((app) => [String(app.id), String(app.permission || '')])
    .sort(([left], [right]) => left.localeCompare(right));
  return JSON.stringify({ role: String(tenant?.role || ''), apps: permissions });
}

function openDatabase() {
  if (databasePromise) return databasePromise;
  databasePromise = new Promise((resolve, reject) => {
    if (!globalThis.indexedDB) {
      reject(new Error('当前浏览器不支持本地 fx 会话存储'));
      return;
    }
    const request = indexedDB.open(databaseName, databaseVersion);
    request.onupgradeneeded = () => {
      const database = request.result;
      if (!database.objectStoreNames.contains(storeName)) database.createObjectStore(storeName, { keyPath: 'key' });
    };
    request.onsuccess = () => {
      const database = request.result;
      database.onversionchange = () => database.close();
      resolve(database);
    };
    request.onerror = () => reject(request.error || new Error('无法打开本地 fx 会话存储'));
    request.onblocked = () => reject(new Error('本地 fx 会话存储正被其他页面占用'));
  }).catch((error) => {
    databasePromise = null;
    throw error;
  });
  return databasePromise;
}

function runTransaction(mode, run) {
  return openDatabase().then((database) => new Promise((resolve, reject) => {
    let value;
    let transaction;
    try {
      transaction = database.transaction(storeName, mode);
      run(transaction.objectStore(storeName), (next) => { value = next; });
    } catch (error) {
      reject(error);
      return;
    }
    transaction.oncomplete = () => resolve(value);
    transaction.onerror = () => reject(transaction.error || new Error('本地 fx 会话存储失败'));
    transaction.onabort = () => reject(transaction.error || new Error('本地 fx 会话存储已中断'));
  }));
}

function storageKey(userId, tenantId) {
  return `${userId}:${tenantId}`;
}

function visibleMessages(messages) {
  let remaining = maxTranscriptCharacters;
  const result = [];
  for (const message of [...(messages || [])].reverse()) {
    if (!['user', 'assistant'].includes(message?.role) || typeof message.content !== 'string') continue;
    const content = message.content.slice(0, Math.min(8192, remaining));
    if (!content) continue;
    result.push({ role: message.role, content });
    remaining -= content.length;
    if (remaining <= 0 || result.length >= maxTranscriptMessages) break;
  }
  return result.reverse();
}

export function createFxConversationStore() {
  function load(userId, tenantId, scope) {
    const key = storageKey(userId, tenantId);
    return runTransaction('readwrite', (store, setValue) => {
      const request = store.get(key);
      request.onsuccess = () => {
        const record = request.result;
        if (!record) return setValue(null);
        if (record.userId !== userId || record.tenantId !== tenantId || record.scope !== scope) {
          store.delete(key);
          setValue(null);
          return;
        }
        if (!(record.checkpoint instanceof Uint8Array) || record.checkpoint.byteLength > maxCheckpointBytes) {
          store.delete(key);
          setValue(null);
          return;
        }
        setValue({
          revision: Number(record.revision) || 0,
          checkpoint: new Uint8Array(record.checkpoint),
          messages: visibleMessages(record.messages),
        });
      };
    });
  }

  function save({ userId, tenantId, scope, expectedRevision, checkpoint, messages }) {
    const bytes = checkpoint instanceof Uint8Array ? new Uint8Array(checkpoint) : new Uint8Array(checkpoint || []);
    if (!bytes.byteLength || bytes.byteLength > maxCheckpointBytes) {
      return Promise.reject(new Error('fx 会话检查点超过本地保存上限'));
    }
    const key = storageKey(userId, tenantId);
    return runTransaction('readwrite', (store, setValue) => {
      const request = store.get(key);
      request.onsuccess = () => {
        const current = request.result;
        const revision = Number(current?.revision) || 0;
        if (current && (current.userId !== userId || current.tenantId !== tenantId || current.scope !== scope)) {
          setValue({ saved: false, changed: true });
          return;
        }
        if (revision !== expectedRevision) {
          setValue({ saved: false, conflict: true });
          return;
        }
        const nextRevision = revision + 1;
        store.put({
          key,
          userId,
          tenantId,
          scope,
          revision: nextRevision,
          updatedAt: Date.now(),
          checkpoint: bytes,
          messages: visibleMessages(messages),
        });
        setValue({ saved: true, revision: nextRevision });
      };
    });
  }

  function clear(userId, tenantId) {
    if (!userId || !tenantId) return Promise.resolve();
    return runTransaction('readwrite', (store) => { store.delete(storageKey(userId, tenantId)); });
  }

  function clearAll() {
    return runTransaction('readwrite', (store) => { store.clear(); });
  }

  function retainAccount(userId) {
    return runTransaction('readwrite', (store) => {
      const request = store.getAll();
      request.onsuccess = () => {
        for (const record of request.result) if (record.userId !== userId) store.delete(record.key);
      };
    });
  }

  function retainWorkspaces(userId, tenantIds) {
    const allowed = new Set(tenantIds);
    return runTransaction('readwrite', (store) => {
      const request = store.getAll();
      request.onsuccess = () => {
        for (const record of request.result) {
          if (record.userId === userId && !allowed.has(record.tenantId)) store.delete(record.key);
        }
      };
    });
  }

  return { load, save, clear, clearAll, retainAccount, retainWorkspaces };
}
