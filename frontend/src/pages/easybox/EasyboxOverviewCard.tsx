import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router';
import { Button, Card, Space, Statistic, Typography } from 'antd';
import type { message } from 'antd';
import { CopyOutlined, PlusOutlined } from '@ant-design/icons';

import { ClipboardManager } from '@/utils';
import { useEasyboxEndpoints, useEasyboxSubscription } from '@/api/queries/useEasybox';

interface EasyboxOverviewCardProps {
  messageApi: ReturnType<typeof message.useMessage>[0];
}

// EasyBox.md §46, §81: the overview answers "are my lines up?" and puts the two
// main actions, Create line and Copy subscription, one click away.
export default function EasyboxOverviewCard({ messageApi }: EasyboxOverviewCardProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const lines = useEasyboxEndpoints();
  const subscription = useEasyboxSubscription();
  const data = lines.data ?? [];
  const running = data.filter((l) => l.status === 'RUNNING').length;
  const failing = data.filter((l) => l.status === 'ERROR').length;

  const copySubscription = async () => {
    const url = subscription.data?.subUrl ?? '';
    if (url && (await ClipboardManager.copyText(url))) messageApi.success(t('copied'));
  };

  return (
    <Card size="small" title={t('easybox.overview.title')} style={{ marginBottom: 16 }}>
      <Space
        wrap
        size="large"
        align="center"
        style={{ width: '100%', justifyContent: 'space-between' }}
      >
        {data.length === 0 && !lines.isLoading ? (
          <Typography.Text type="secondary">{t('easybox.overview.empty')}</Typography.Text>
        ) : (
          <Space size="large">
            <Statistic title={t('easybox.overview.total')} value={data.length} />
            <Statistic title={t('easybox.status.RUNNING')} value={running} />
            <Statistic
              title={t('easybox.status.ERROR')}
              value={failing}
              valueStyle={failing > 0 ? { color: 'var(--ant-color-error)' } : undefined}
            />
          </Space>
        )}
        <Space wrap>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => navigate('/easybox/create')}
          >
            {t('easybox.lines.create')}
          </Button>
          <Button
            icon={<CopyOutlined />}
            disabled={!subscription.data?.subUrl}
            onClick={copySubscription}
          >
            {t('easybox.create.copySubscription')}
          </Button>
        </Space>
      </Space>
    </Card>
  );
}
