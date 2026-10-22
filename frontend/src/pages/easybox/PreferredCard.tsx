import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Card, Input, Space, Typography } from 'antd';
import { DeleteOutlined, PlusOutlined, ThunderboltOutlined } from '@ant-design/icons';

import { useEasyboxPreferred, useEasyboxPreferredMutation } from '@/api/queries/useEasybox';
import type { PreferredAddress } from '@/schemas/api/easybox';
import type { EasyboxPageApi } from './EasyboxShell';

const SPEEDTEST_PAGE = 'https://github.com/XIU2/CloudflareSpeedTest/releases';
// Mirror the backend limits (internal/easybox/cdn_preferred.go).
const MAX_PREFERRED = 10;
const MAX_REMARK = 24;

/* Preferred Cloudflare addresses: extra "<line>-<remark>" nodes for every line
 * that goes through Cloudflare. Independent of the Cloudflare token. */
export default function PreferredCard({ messageApi }: EasyboxPageApi) {
  const { t } = useTranslation();
  const query = useEasyboxPreferred();
  const { save, saving } = useEasyboxPreferredMutation();
  // null until the admin edits: the rows show what is saved.
  const [draft, setDraft] = useState<PreferredAddress[] | null>(null);
  const [error, setError] = useState('');

  const saved = query.data?.addresses ?? [];
  const rows = draft ?? saved;
  const updateRow = (i: number, patch: Partial<PreferredAddress>) =>
    setDraft(rows.map((row, j) => (j === i ? { ...row, ...patch } : row)));

  const submit = async () => {
    setError('');
    const res = await save(rows.filter((row) => row.address.trim() || row.remark.trim()));
    if (!res.ok) {
      setError(res.message);
      return;
    }
    setDraft(null);
    messageApi.success(t('easybox.cloudflare.preferredSaved'));
  };

  return (
    <Card
      size="small"
      title={
        <Space>
          <ThunderboltOutlined />
          {t('easybox.cloudflare.preferredTitle')}
        </Space>
      }
    >
      <Space direction="vertical" style={{ width: '100%' }}>
        <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
          {t('easybox.cloudflare.preferredHelp')}{' '}
          <Typography.Link href={SPEEDTEST_PAGE} target="_blank" rel="noreferrer">
            CloudflareSpeedTest
          </Typography.Link>
        </Typography.Paragraph>
        {rows.map((row, i) => (
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
              onClick={() => setDraft(rows.filter((_, j) => j !== i))}
            />
          </Space.Compact>
        ))}
        {error && <Alert type="error" showIcon message={error} />}
        <Space wrap>
          <Button
            icon={<PlusOutlined />}
            disabled={rows.length >= MAX_PREFERRED}
            onClick={() => setDraft([...rows, { address: '', remark: '' }])}
          >
            {t('easybox.cloudflare.preferredAdd', { max: MAX_PREFERRED })}
          </Button>
          <Button
            type="primary"
            loading={saving}
            disabled={JSON.stringify(rows) === JSON.stringify(saved)}
            onClick={submit}
          >
            {t('easybox.cloudflare.preferredSave')}
          </Button>
        </Space>
      </Space>
    </Card>
  );
}
