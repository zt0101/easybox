import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Input, Space, Typography } from 'antd';

import { useEasyboxRegion, useEasyboxRegionMutation } from '@/api/queries/useEasybox';
import type { EasyboxPageApi } from './EasyboxShell';

/* The region code automatic line names start with (HK-VLESS-Reality):
 * detected from the server IP, editable when the detection is wrong. */
export default function RegionField({ messageApi }: EasyboxPageApi) {
  const { t } = useTranslation();
  const region = useEasyboxRegion();
  const { saveRegion, savingRegion } = useEasyboxRegionMutation();
  const [draft, setDraft] = useState<string | null>(null);

  const code = region.data?.code ?? '';
  const save = async () => {
    const res = await saveRegion((draft ?? '').trim());
    if (!res.ok) {
      messageApi.error(res.message);
      return;
    }
    setDraft(null);
    messageApi.success(t('easybox.create.regionSaved'));
  };

  return (
    <Space direction="vertical" size={2} style={{ marginBottom: 16 }}>
      <Space wrap>
        <Typography.Text>{t('easybox.create.region')}:</Typography.Text>
        {draft === null ? (
          <>
            <Typography.Text strong>{code || t('easybox.create.regionUnknown')}</Typography.Text>
            <Button size="small" onClick={() => setDraft(code)}>
              {t('easybox.create.regionEdit')}
            </Button>
          </>
        ) : (
          <>
            <Input
              size="small"
              value={draft}
              onChange={(e) => setDraft(e.target.value.toUpperCase())}
              onPressEnter={save}
              maxLength={8}
              style={{ width: 96 }}
              dir="ltr"
              autoFocus
            />
            <Button size="small" type="primary" loading={savingRegion} onClick={save}>
              {t('easybox.create.regionSave')}
            </Button>
            <Button size="small" onClick={() => setDraft(null)}>
              {t('cancel')}
            </Button>
          </>
        )}
      </Space>
      <Typography.Text type="secondary">{t('easybox.create.regionHelp')}</Typography.Text>
    </Space>
  );
}
