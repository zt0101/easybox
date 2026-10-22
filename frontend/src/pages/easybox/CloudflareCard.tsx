import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Card, Form, Input, Space, Typography } from 'antd';
import { CloudOutlined, DeleteOutlined, PlusOutlined } from '@ant-design/icons';

import { useEasyboxCloudflare, useEasyboxCloudflareMutations } from '@/api/queries/useEasybox';
import type { PreferredAddress } from '@/schemas/api/easybox';
import type { EasyboxPageApi } from './EasyboxShell';

const TOKEN_PAGE = 'https://dash.cloudflare.com/profile/api-tokens';
const SPEEDTEST_PAGE = 'https://github.com/XIU2/CloudflareSpeedTest/releases';
// Mirror the backend limits (internal/easybox/cdn_preferred.go).
const MAX_PREFERRED = 10;
const MAX_REMARK = 24;

/* CDN lines need a Cloudflare API token and the domain it manages. The token
 * is write-only: the panel shows only its last characters. */
export default function CloudflareCard({ messageApi, modal }: EasyboxPageApi) {
  const { t } = useTranslation();
  const cloudflare = useEasyboxCloudflare();
  const { save, saving, clear, savePreferred, savingPreferred } = useEasyboxCloudflareMutations();
  const [editing, setEditing] = useState(false);
  const [token, setToken] = useState('');
  const [domain, setDomain] = useState('');
  const [error, setError] = useState('');
  // null until the admin edits: the rows show what is saved.
  const [draft, setDraft] = useState<PreferredAddress[] | null>(null);
  const [preferredError, setPreferredError] = useState('');

  const view = cloudflare.data;
  const showForm = editing || !view?.configured;
  const saved = view?.preferredAddresses ?? [];
  const preferred = draft ?? saved;
  const updateRow = (i: number, patch: Partial<PreferredAddress>) =>
    setDraft(preferred.map((row, j) => (j === i ? { ...row, ...patch } : row)));

  const submitPreferred = async () => {
    setPreferredError('');
    const res = await savePreferred(
      preferred.filter((row) => row.address.trim() || row.remark.trim()),
    );
    if (!res.ok) {
      setPreferredError(res.message);
      return;
    }
    setDraft(null);
    messageApi.success(t('easybox.cloudflare.preferredSaved'));
  };

  const submit = async () => {
    setError('');
    const res = await save({ token, domain });
    if (!res.ok) {
      setError(res.message);
      return;
    }
    setToken('');
    setEditing(false);
    messageApi.success(t('easybox.cloudflare.saved'));
  };

  const disconnect = () =>
    modal.confirm({
      title: t('easybox.cloudflare.clearTitle'),
      content: t('easybox.cloudflare.clearContent'),
      okText: t('easybox.cloudflare.clear'),
      okButtonProps: { danger: true },
      cancelText: t('cancel'),
      onOk: () => clear(),
    });

  return (
    <Card
      size="small"
      title={
        <Space>
          <CloudOutlined />
          {t('easybox.cloudflare.title')}
        </Space>
      }
    >
      {view?.configured && !editing ? (
        <Space wrap>
          <Typography.Text>
            {t('easybox.cloudflare.connected', { domain: view.domain, hint: view.tokenHint })}
          </Typography.Text>
          <Button size="small" onClick={() => setEditing(true)}>
            {t('easybox.cloudflare.change')}
          </Button>
          <Button size="small" danger onClick={disconnect}>
            {t('easybox.cloudflare.clear')}
          </Button>
        </Space>
      ) : null}
      {view?.configured && !editing && (
        <Space direction="vertical" style={{ width: '100%', marginTop: 16 }}>
          <Typography.Text strong>{t('easybox.cloudflare.preferredTitle')}</Typography.Text>
          <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
            {t('easybox.cloudflare.preferredHelp')}{' '}
            <Typography.Link href={SPEEDTEST_PAGE} target="_blank" rel="noreferrer">
              CloudflareSpeedTest
            </Typography.Link>
          </Typography.Paragraph>
          {preferred.map((row, i) => (
            <Space.Compact key={i} style={{ width: '100%' }}>
              <Input
                value={row.address}
                onChange={(e) => updateRow(i, { address: e.target.value })}
                placeholder="104.17.138.161"
                aria-label={t('easybox.cloudflare.preferredAddress')}
                maxLength={253}
                dir="ltr"
                style={{ flex: 3 }}
              />
              <Input
                value={row.remark}
                onChange={(e) => updateRow(i, { remark: e.target.value })}
                placeholder={t('easybox.cloudflare.preferredRemarkPlaceholder')}
                aria-label={t('easybox.cloudflare.preferredRemark')}
                maxLength={MAX_REMARK}
                style={{ flex: 2 }}
              />
              <Button
                icon={<DeleteOutlined />}
                aria-label={t('delete')}
                onClick={() => setDraft(preferred.filter((_, j) => j !== i))}
              />
            </Space.Compact>
          ))}
          {preferredError && <Alert type="error" showIcon message={preferredError} />}
          <Space wrap>
            <Button
              icon={<PlusOutlined />}
              disabled={preferred.length >= MAX_PREFERRED}
              onClick={() => setDraft([...preferred, { address: '', remark: '' }])}
            >
              {t('easybox.cloudflare.preferredAdd', { max: MAX_PREFERRED })}
            </Button>
            <Button
              type="primary"
              loading={savingPreferred}
              disabled={JSON.stringify(preferred) === JSON.stringify(saved)}
              onClick={submitPreferred}
            >
              {t('easybox.cloudflare.preferredSave')}
            </Button>
          </Space>
        </Space>
      )}
      {showForm && (
        <Space direction="vertical" style={{ width: '100%' }}>
          <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
            {t('easybox.cloudflare.intro')}
          </Typography.Paragraph>
          <ol className="easybox-cloudflare-steps">
            <li>{t('easybox.cloudflare.step1')}</li>
            <li>
              {t('easybox.cloudflare.step2')}{' '}
              <Typography.Link href={TOKEN_PAGE} target="_blank" rel="noreferrer">
                {t('easybox.cloudflare.openTokenPage')}
              </Typography.Link>
            </li>
            <li>{t('easybox.cloudflare.step3')}</li>
          </ol>
          <Form layout="vertical" component={false}>
            <Form.Item label={t('easybox.cloudflare.domain')} style={{ marginBottom: 12 }}>
              <Input
                value={domain}
                onChange={(e) => setDomain(e.target.value)}
                placeholder="example.com"
                maxLength={253}
                dir="ltr"
              />
            </Form.Item>
            <Form.Item label={t('easybox.cloudflare.token')} style={{ marginBottom: 12 }}>
              <Input.Password
                value={token}
                onChange={(e) => setToken(e.target.value)}
                autoComplete="off"
                maxLength={200}
                dir="ltr"
              />
            </Form.Item>
          </Form>
          {error && <Alert type="error" showIcon message={error} />}
          <Space>
            <Button
              type="primary"
              loading={saving}
              disabled={!token.trim() || !domain.trim()}
              onClick={submit}
            >
              {t('easybox.cloudflare.save')}
            </Button>
            {editing && <Button onClick={() => setEditing(false)}>{t('cancel')}</Button>}
          </Space>
        </Space>
      )}
    </Card>
  );
}
