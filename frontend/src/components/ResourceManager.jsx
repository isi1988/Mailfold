import React, { useState } from 'react';
import { PageHeader } from '../ds/components/molecules/PageHeader.jsx';
import { SearchInput } from '../ds/components/molecules/SearchInput.jsx';
import { Table, TableRow } from '../ds/components/organisms/Table.jsx';
import { Pagination } from '../ds/components/molecules/Pagination.jsx';
import { Icon } from '../ds/components/atoms/Icon.jsx';
import { Button } from '../ds/components/atoms/Button.jsx';
import { Drawer } from '../ds/components/organisms/Drawer.jsx';
import { FormField } from '../ds/components/molecules/FormField.jsx';
import { Input } from '../ds/components/atoms/Input.jsx';
import { Toggle } from '../ds/components/atoms/Toggle.jsx';
import { ConfirmModal } from '../ds/components/organisms/ConfirmModal.jsx';
import { useApi } from '../lib/useApi.js';
import { api } from '../api/client.js';
import { AsyncView } from '../components/States.jsx';
import { useToast } from '../components/Toast.jsx';
import { asList } from '../lib/format.js';
import { useT } from '../i18n/index.jsx';

const PAGE_SIZE = 20;

function errText(err, fallback) {
  return (err && err.body && err.body.message) || (err && err.message) || fallback;
}

// coerce turns a form value into the shape mailcow expects for the field type.
function coerce(field, value) {
  if (field.type === 'toggle') return value ? '1' : '0';
  if (field.type === 'number') return Number(value) || 0;
  return value;
}

// initialFor derives a field's starting value for create (default) or edit (row).
function initialFor(field, row) {
  if (row && field.key in row) {
    if (field.type === 'toggle') return row[field.key] === 1 || row[field.key] === '1' || row[field.key] === true;
    return row[field.key];
  }
  if (field.type === 'toggle') return field.default != null ? field.default : true;
  if (field.type === 'number') return field.default != null ? field.default : 0;
  if (field.type === 'select') return field.default != null ? field.default : (field.options && field.options[0] ? field.options[0].value : '');
  return field.default != null ? field.default : '';
}

// ResourceDrawer is the generic create/edit form for one resource row.
function ResourceDrawer({ mode, row, fields, title, subtitle, onClose, onSubmit, onDelete, busy }) {
  const t = useT();
  const editing = mode === 'edit';
  const [values, setValues] = useState(() => {
    const v = {};
    fields.forEach(f => { v[f.key] = initialFor(f, editing ? row : null); });
    return v;
  });
  const set = (k, val) => setValues(v => ({ ...v, [k]: val }));

  const footer = (
    <>
      {editing && onDelete && <Button variant="danger" onClick={() => onDelete(row)}>{t('common.delete')}</Button>}
      <Button variant="secondary" className="mf-spacer" onClick={onClose}>{t('common.cancel')}</Button>
      <Button variant="primary" onClick={() => onSubmit(values)} disabled={busy}>
        {busy ? t('common.saving') : (editing ? t('common.save') : t('common.create'))}
      </Button>
    </>
  );

  return (
    <Drawer title={title} subtitle={subtitle} footer={footer} onClose={onClose}>
      {fields.map(f => {
        // In edit mode, fields flagged createOnly are read-only identifiers.
        const locked = editing && f.createOnly;
        if (f.type === 'toggle') {
          return (
            <div key={f.key} className="mf-row mf-row--between" style={{ marginTop: 8 }}>
              <span className="mf-u-muted" style={{ fontSize: 13 }}>{f.label}</span>
              <Toggle on={!!values[f.key]} onClick={() => !locked && set(f.key, !values[f.key])} style={{ cursor: locked ? 'default' : 'pointer' }} />
            </div>
          );
        }
        return (
          <FormField key={f.key} label={f.label}>
            {f.type === 'select' ? (
              <select className="mf-input" value={values[f.key]} disabled={locked} onChange={e => set(f.key, e.target.value)}>
                {(f.options || []).map(o => <option key={o.value} value={o.value}>{o.label}</option>)}
              </select>
            ) : f.type === 'textarea' ? (
              <textarea className="mf-input" rows={f.rows || 3} placeholder={f.placeholder} value={values[f.key]} readOnly={locked} onChange={e => set(f.key, e.target.value)} />
            ) : (
              <Input
                type={f.type === 'number' ? 'number' : 'text'}
                mono={f.mono}
                placeholder={f.placeholder}
                value={values[f.key]}
                readonly={locked}
                onChange={e => set(f.key, f.type === 'number' ? (Number(e.target.value) || 0) : e.target.value)}
              />
            )}
            {f.hint ? <div className="mf-u-faint" style={{ fontSize: 12, marginTop: 4 }}>{f.hint}</div> : null}
          </FormField>
        );
      })}
    </Drawer>
  );
}

/**
 * A generic list + create/edit drawer + delete manager for a mailcow CRUD
 * resource, so the many small "advanced" resources are written once.
 *
 *   endpoint    '/api/relayhosts'
 *   idKey       field used as the identifier for edit/delete (default 'id')
 *   title/sub   page header strings
 *   addLabel    "+ Add ..." button text
 *   filterKeys  row keys the search box matches against (optional)
 *   filterPlaceholder
 *   columns     [{ key, label, w, mono, render(row) }]
 *   fields      [{ key, label, type:'text'|'number'|'select'|'toggle'|'textarea', options, placeholder, hint, default, createOnly, mono, editable:false }]
 *   canCreate / canEdit / canDelete
 *   labels      { deleteTitle, deleteMsg(row), created, updated, deleted, failed, empty }
 *   describe(row) -> string  (identifier shown in toasts / drawer subtitle)
 */
export function ResourceManager({
  endpoint, idKey = 'id', title, sub, addLabel,
  filterKeys = [], filterPlaceholder,
  columns = [], fields = [],
  canCreate = true, canEdit = true, canDelete = true,
  labels = {}, describe,
}) {
  const t = useT();
  const { toast } = useToast();
  const { data, loading, error, reload } = useApi(endpoint, []);
  const [q, setQ] = useState('');
  const [page, setPage] = useState(1);
  const [drawer, setDrawer] = useState(null); // { mode, row }
  const [confirm, setConfirm] = useState(null);
  const [busy, setBusy] = useState(false);

  const idOf = row => row[idKey];
  const nameOf = row => (describe ? describe(row) : String(idOf(row) ?? ''));

  const rows = asList(data);
  const filtered = q && filterKeys.length
    ? rows.filter(r => filterKeys.some(k => String(r[k] ?? '').toLowerCase().includes(q.toLowerCase())))
    : rows;
  const pageCount = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE));
  const current = Math.min(page, pageCount);
  const paged = filtered.slice((current - 1) * PAGE_SIZE, current * PAGE_SIZE);
  const from = filtered.length === 0 ? 0 : (current - 1) * PAGE_SIZE + 1;
  const to = Math.min(current * PAGE_SIZE, filtered.length);

  async function submit(values) {
    setBusy(true);
    try {
      if (drawer.mode === 'edit') {
        const attr = {};
        fields.forEach(f => { if (!f.createOnly && f.editable !== false) attr[f.key] = coerce(f, values[f.key]); });
        await api.put(endpoint, { items: [idOf(drawer.row)], attr });
        toast(labels.updated || t('common.saved'));
      } else {
        const body = {};
        fields.forEach(f => { body[f.key] = coerce(f, values[f.key]); });
        await api.post(endpoint, body);
        toast(labels.created || t('common.created'));
      }
      setDrawer(null);
      reload();
    } catch (err) {
      toast(labels.failed || t('common.failed'), errText(err, ''));
    } finally {
      setBusy(false);
    }
  }

  async function doDelete() {
    const row = confirm;
    setConfirm(null);
    try {
      await api.del(endpoint, { items: [idOf(row)] });
      toast(labels.deleted || t('common.deleted'));
      reload();
    } catch (err) {
      toast(labels.failed || t('common.failed'), errText(err, ''));
    }
  }

  const cols = [...columns];
  if (canDelete || canEdit) cols.push({ label: '', w: '18px' });

  return (
    <>
      <PageHeader
        title={title}
        sub={sub}
        actions={canCreate ? <Button variant="primary" onClick={() => setDrawer({ mode: 'create' })}>{addLabel || t('common.create')}</Button> : null}
      />
      {filterKeys.length > 0 && (
        <div className="mf-row" style={{ marginBottom: 14 }}>
          <SearchInput className="mf-spacer" style={{ width: 250 }} placeholder={filterPlaceholder || ''} value={q} onChange={e => { setQ(e.target.value); setPage(1); }} />
        </div>
      )}

      <AsyncView loading={loading} error={error} reload={reload} empty={filtered.length === 0 ? (labels.empty || t('states.empty')) : null}>
        <Table columns={cols}>
          {paged.map((row, i) => (
            <TableRow
              key={idOf(row) ?? i}
              onClick={canEdit ? () => setDrawer({ mode: 'edit', row }) : undefined}
              style={canEdit ? { cursor: 'pointer' } : undefined}
            >
              {columns.map(c => (
                <span key={c.key} className={c.mono ? 'mf-u-mono mf-truncate' : 'mf-truncate'} style={{ fontSize: 13, color: 'var(--ink)' }}>
                  {c.render ? c.render(row) : (row[c.key] ?? '—')}
                </span>
              ))}
              {(canDelete || canEdit) && (
                canDelete
                  ? <Button variant="ghost" size="sm" title={t('common.delete')} onClick={e => { e.stopPropagation(); setConfirm(row); }}><Icon name="trash" size={15} /></Button>
                  : <Icon name="chevron-right" size={14} style={{ color: 'var(--faint)' }} />
              )}
            </TableRow>
          ))}
        </Table>
        {filtered.length > 0 && (
          <div style={{ marginTop: 16 }}>
            <Pagination page={current} pageCount={pageCount} summary={t('common.showing', { from, to, total: filtered.length })} onPage={setPage} />
          </div>
        )}
      </AsyncView>

      {drawer && (
        <ResourceDrawer
          mode={drawer.mode}
          row={drawer.row}
          fields={fields}
          title={drawer.mode === 'edit' ? (labels.editTitle || t('common.edit')) : (labels.newTitle || t('common.create'))}
          subtitle={drawer.mode === 'edit' ? nameOf(drawer.row) : (sub || '')}
          busy={busy}
          onClose={() => setDrawer(null)}
          onSubmit={submit}
          onDelete={canDelete ? row => { setDrawer(null); setConfirm(row); } : undefined}
        />
      )}
      {confirm && (
        <ConfirmModal
          title={labels.deleteTitle || t('common.delete')}
          msg={labels.deleteMsg ? labels.deleteMsg(nameOf(confirm)) : nameOf(confirm)}
          cta={t('common.delete')}
          danger
          onCancel={() => setConfirm(null)}
          onConfirm={doDelete}
        />
      )}
    </>
  );
}
