import { useTranslation } from 'react-i18next';
import { Descriptions, Tag } from 'antd';

export const TIER_COLORS: Record<string, string> = {
  recommended: 'green',
  backup: 'blue',
  risky: 'orange',
};

export function TierTag({ tier }: { tier: string }) {
  const { t } = useTranslation();
  if (!TIER_COLORS[tier]) return null;
  return (
    <Tag color={TIER_COLORS[tier]} style={{ marginInlineEnd: 0 }}>
      {t(`easybox.tiers.${tier}`)}
    </Tag>
  );
}

const ROWS = ['how', 'pros', 'cons', 'use', 'clients'] as const;

/* The selected protocol explained: how it works, trade-offs, when to use it
 * and which clients were measured to connect (CLIENT_COMPATIBILITY.md). */
export default function TemplateDetails({
  id,
  name,
  tier,
}: {
  id: string;
  name: string;
  tier: string;
}) {
  const { t } = useTranslation();
  return (
    <Descriptions
      bordered
      size="small"
      column={1}
      title={
        <span>
          {name} <TierTag tier={tier} />
        </span>
      }
      items={ROWS.map((row) => ({
        key: row,
        label: t(`easybox.templateDetails.labels.${row}`),
        children: t(`easybox.templateDetails.${id}.${row}`),
      }))}
    />
  );
}
