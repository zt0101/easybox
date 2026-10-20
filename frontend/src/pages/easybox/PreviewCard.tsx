import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Card, Segmented, Space, Spin, Tag, Typography } from 'antd';
import { EyeOutlined } from '@ant-design/icons';

import { useEasyboxPreview } from '@/api/queries/useEasybox';

// The target clients of EasyBox.md §32, by their own product names.
const CLIENTS = [
  { value: 'v2rayn', label: 'v2rayN' },
  { value: 'shadowrocket', label: 'Shadowrocket' },
  { value: 'passwall', label: 'PassWall' },
  { value: 'singbox', label: 'sing-box' },
  { value: 'mihomo', label: 'Mihomo' },
];

// EasyBox.md §43: lets the admin see exactly what one client receives.
export default function PreviewCard() {
  const { t } = useTranslation();
  const [client, setClient] = useState<string | null>(null);
  const preview = useEasyboxPreview(client);

  return (
    <Card
      title={
        <Space>
          <EyeOutlined />
          {t('easybox.subscription.preview')}
        </Space>
      }
    >
      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
        <Typography.Text type="secondary">{t('easybox.subscription.previewHint')}</Typography.Text>
        <Segmented
          options={CLIENTS}
          value={client ?? undefined}
          onChange={(v) => setClient(String(v))}
        />
        {client && preview.isLoading && <Spin />}
        {preview.isError && (
          <Alert type="error" showIcon message={(preview.error as Error).message} />
        )}
        {client && preview.data && (
          <>
            <Tag>{t('easybox.subscription.previewFormat', { format: preview.data.format })}</Tag>
            {preview.data.skipped.length > 0 && (
              <Alert
                type="warning"
                showIcon
                message={t('easybox.subscription.previewSkipped')}
                description={
                  <ul style={{ margin: 0, paddingInlineStart: 18 }}>
                    {preview.data.skipped.map((s) => (
                      <li key={s} className="easybox-mono">
                        {s}
                      </li>
                    ))}
                  </ul>
                }
              />
            )}
            {preview.data.body ? (
              <pre className="easybox-mono easybox-preview" dir="ltr">
                {preview.data.body}
              </pre>
            ) : (
              <Typography.Text type="secondary">
                {t('easybox.subscription.previewEmpty')}
              </Typography.Text>
            )}
          </>
        )}
      </Space>
    </Card>
  );
}
