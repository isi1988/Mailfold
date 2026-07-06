import React, { useState } from 'react';
import { PageHeader } from '../ds/components/molecules/PageHeader.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { Input } from '../ds/components/atoms/Input.jsx';
import { FormField } from '../ds/components/molecules/FormField.jsx';
import { Icon } from '../ds/components/atoms/Icon.jsx';
import { Chip } from '../ds/components/atoms/Chip.jsx';
import { Table, TableRow } from '../ds/components/organisms/Table.jsx';
import { Drawer } from '../ds/components/organisms/Drawer.jsx';
import { ConfirmModal } from '../ds/components/organisms/ConfirmModal.jsx';
import { AsyncView } from '../components/States.jsx';
import { useApi } from '../lib/useApi.js';
import { asList } from '../lib/format.js';
import { api } from '../api/client.js';
import { useToast } from '../components/Toast.jsx';
import { useT } from '../i18n/index.jsx';

function errText(err, fallback) {
  return (err && err.body && err.body.error) || (err && err.message) || fallback;
}

// CreateDrawer registers an existing mailcow mailbox as a shared/team
// mailbox. Members are added afterward, from ManageDrawer below, once the
// mailbox has an id.
function CreateDrawer({ onClose, onSaved }) {
  const t = useT();
  const { toast } = useToast();
  const [email, setEmail] = useState('');
  const [displayName, setDisplayName] = useState('');
  const [busy, setBusy] = useState(false);

  async function save() {
    if (busy || !email.trim()) return;
    setBusy(true);
    try {
      await api.post('/api/shared-mailboxes', { email: email.trim(), display_name: displayName.trim() });
      toast(t('sharedMailboxes.created'));
      onSaved();
      onClose();
    } catch (err) {
      toast(t('sharedMailboxes.failed'), errText(err, ''));
    } finally {
      setBusy(false);
    }
  }

  const footer = (
    <>
      <Button variant="secondary" className="mf-spacer" onClick={onClose}>{t('common.cancel')}</Button>
      <Button variant="primary" onClick={save} disabled={busy || !email.trim()}>{busy ? t('common.saving') : t('common.create')}</Button>
    </>
  );

  return (
    <Drawer title={t('sharedMailboxes.newTitle')} subtitle={t('sharedMailboxes.sub')} footer={footer} onClose={onClose}>
      <FormField label={t('sharedMailboxes.email')}>
        <Input mono value={email} onChange={e => setEmail(e.target.value)} placeholder="support@example.com" />
      </FormField>
      <FormField label={t('sharedMailboxes.displayName')}>
        <Input value={displayName} onChange={e => setDisplayName(e.target.value)} placeholder={t('sharedMailboxes.displayNamePlaceholder')} />
      </FormField>
      <div className="mf-u-faint" style={{ fontSize: 12, marginTop: 4, lineHeight: 1.5 }}>{t('sharedMailboxes.createHint')}</div>
    </Drawer>
  );
}

// ManageDrawer adds/removes members of an existing shared mailbox.
function ManageDrawer({ mailbox, onClose, onChanged }) {
  const t = useT();
  const { toast } = useToast();
  const [members, setMembers] = useState(mailbox.members || []);
  const [newMember, setNewMember] = useState('');
  const [busy, setBusy] = useState(false);

  async function addMember() {
    const email = newMember.trim();
    if (busy || !email || members.includes(email)) return;
    setBusy(true);
    try {
      await api.post('/api/shared-mailboxes/members', { mailbox_id: mailbox.id, email });
      setMembers(list => [...list, email]);
      setNewMember('');
      onChanged();
    } catch (err) {
      toast(t('sharedMailboxes.failed'), errText(err, ''));
    } finally {
      setBusy(false);
    }
  }

  async function removeMember(email) {
    try {
      await api.del('/api/shared-mailboxes/members', { mailbox_id: mailbox.id, email });
      setMembers(list => list.filter(m => m !== email));
      onChanged();
    } catch (err) {
      toast(t('sharedMailboxes.failed'), errText(err, ''));
    }
  }

  return (
    <Drawer title={mailbox.display_name || mailbox.email} subtitle={mailbox.email} onClose={onClose}>
      <div className="mf-u-faint" style={{ fontSize: 12.5, marginBottom: 14, lineHeight: 1.5 }}>{t('sharedMailboxes.membersHint')}</div>
      <div className="mf-row" style={{ gap: 8, marginBottom: 14 }}>
        <Input className="mf-spacer" placeholder="alice@example.com" value={newMember}
          onChange={e => setNewMember(e.target.value)}
          onKeyDown={e => { if (e.key === 'Enter') addMember(); }} />
        <Button variant="secondary" size="sm" onClick={addMember} disabled={busy || !newMember.trim()}>{t('sharedMailboxes.addMember')}</Button>
      </div>
      {members.length === 0 ? (
        <div className="mf-u-faint" style={{ fontSize: 12.5 }}>{t('sharedMailboxes.noMembers')}</div>
      ) : members.map(email => (
        <div key={email} className="mf-row mf-row--between" style={{ padding: '7px 0', borderTop: '1px solid var(--hair-soft)' }}>
          <span className="mf-u-mono mf-truncate" style={{ fontSize: 13 }}>{email}</span>
          <Button variant="ghost" size="sm" title={t('common.delete')} onClick={() => removeMember(email)}>
            <Icon name="trash" size={15} />
          </Button>
        </div>
      ))}
    </Drawer>
  );
}

export function SharedMailboxesPage() {
  const t = useT();
  const { toast } = useToast();
  const { data, loading, error, reload } = useApi('/api/shared-mailboxes', []);
  const [creating, setCreating] = useState(false);
  const [managing, setManaging] = useState(null);
  const [confirmDelete, setConfirmDelete] = useState(null);

  const rows = asList(data);

  async function doDelete() {
    const m = confirmDelete;
    setConfirmDelete(null);
    try {
      await api.del('/api/shared-mailboxes', { id: String(m.id) });
      toast(t('sharedMailboxes.deleted'));
      reload();
    } catch (err) {
      toast(t('sharedMailboxes.failed'), errText(err, ''));
    }
  }

  return (
    <>
      <PageHeader
        title={t('sharedMailboxes.title')}
        sub={t('sharedMailboxes.sub')}
        actions={<Button variant="primary" onClick={() => setCreating(true)}>{t('sharedMailboxes.add')}</Button>}
      />
      <AsyncView loading={loading} error={error} reload={reload} empty={rows.length === 0 ? t('sharedMailboxes.empty') : null}>
        <Table columns={[
          { label: t('sharedMailboxes.col.mailbox'), w: '1.5fr' },
          { label: t('sharedMailboxes.col.members'), w: '2fr' },
          { label: t('sharedMailboxes.col.createdBy'), w: '1fr' },
          { label: '', w: '18px' },
        ]}>
          {rows.map(m => (
            <TableRow key={m.id} onClick={() => setManaging(m)} style={{ cursor: 'pointer' }}>
              <div className="mf-min0">
                <div className="mf-truncate" style={{ fontSize: 13, color: 'var(--ink)' }}>{m.display_name || m.email}</div>
                <div className="mf-u-faint mf-u-mono mf-truncate" style={{ fontSize: 12 }}>{m.email}</div>
              </div>
              <div style={{ display: 'flex', gap: 4, flexWrap: 'wrap' }}>
                {(m.members || []).length === 0
                  ? <span className="mf-u-faint" style={{ fontSize: 12.5 }}>{t('sharedMailboxes.noMembers')}</span>
                  : m.members.map(email => <Chip key={email} style={{ fontSize: 11 }}>{email}</Chip>)}
              </div>
              <span className="mf-u-faint" style={{ fontSize: 12.5 }}>{m.created_by || '—'}</span>
              <Button variant="ghost" size="sm" title={t('common.delete')} onClick={e => { e.stopPropagation(); setConfirmDelete(m); }}>
                <Icon name="trash" size={15} />
              </Button>
            </TableRow>
          ))}
        </Table>
      </AsyncView>
      {creating && <CreateDrawer onClose={() => setCreating(false)} onSaved={reload} />}
      {managing && (
        <ManageDrawer mailbox={managing} onClose={() => setManaging(null)} onChanged={reload} />
      )}
      {confirmDelete && (
        <ConfirmModal
          title={t('sharedMailboxes.deleteTitle')}
          msg={t('sharedMailboxes.deleteMsg', { name: confirmDelete.display_name || confirmDelete.email })}
          cta={t('common.delete')}
          danger
          onCancel={() => setConfirmDelete(null)}
          onConfirm={doDelete}
        />
      )}
    </>
  );
}
