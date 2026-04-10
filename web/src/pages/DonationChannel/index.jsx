/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useContext, useEffect, useMemo, useState } from 'react';
import {
  Banner,
  Button,
  Card,
  Form,
  Radio,
  RadioGroup,
  Space,
  Spin,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import {
  API,
  copy,
  showError,
  showSuccess,
  timestamp2string,
  renderQuotaWithPrompt,
} from '../../helpers';
import { useTranslation } from 'react-i18next';
import { StatusContext } from '../../context/Status';

export default function DonationChannel() {
  const { t } = useTranslation();
  const [statusState] = useContext(StatusContext);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [channels, setChannels] = useState([]);
  const [leaderboard, setLeaderboard] = useState([]);
  const [scope, setScope] = useState('all');
  const [formApi, setFormApi] = useState(null);
  const donationGuideText = statusState?.status?.donation_guide_text || '';
  const donationCopyButtonText =
    statusState?.status?.donation_copy_button_text || '';
  const donationCopyButtonContent =
    statusState?.status?.donation_copy_button_content || '';
  const shouldShowCopyButton =
    donationCopyButtonText.trim() !== '' &&
    donationCopyButtonContent.trim() !== '';

  const filteredChannels = useMemo(() => {
    if (scope === 'mine') {
      return channels.filter((channel) => channel.is_mine);
    }
    return channels;
  }, [channels, scope]);

  const columns = useMemo(
    () => [
      {
        title: t('名称'),
        dataIndex: 'name',
      },
      {
        title: t('状态'),
        dataIndex: 'status',
        render: (status) => (
          <Tag color={status === 1 ? 'green' : 'grey'}>
            {status === 1 ? t('启用') : t('禁用')}
          </Tag>
        ),
      },
      {
        title: t('创建时间'),
        dataIndex: 'created_time',
        render: (value) => timestamp2string(value),
      },
    ],
    [t],
  );

  const leaderboardColumns = useMemo(
    () => [
      {
        title: t('排名'),
        dataIndex: 'rank',
        render: (_, __, index) => index + 1,
      },
      {
        title: t('用户'),
        dataIndex: 'username',
        render: (_, record) => {
          const displayName = record.display_name?.trim();
          if (displayName) {
            return `${displayName} (@${record.username})`;
          }
          return record.username;
        },
      },
      {
        title: t('捐赠次数'),
        dataIndex: 'donation_count',
      },
    ],
    [t],
  );

  const loadChannels = async () => {
    setLoading(true);
    try {
      const [channelsRes, leaderboardRes] = await Promise.all([
        API.get('/api/user/donation-channel'),
        API.get('/api/user/donation-channel/leaderboard'),
      ]);
      const channelsPayload = channelsRes.data;
      if (!channelsPayload.success) {
        showError(channelsPayload.message);
        return;
      }
      const leaderboardPayload = leaderboardRes.data;
      if (!leaderboardPayload.success) {
        showError(leaderboardPayload.message);
        return;
      }
      setChannels(channelsPayload.data || []);
      setLeaderboard(leaderboardPayload.data || []);
    } catch (error) {
      showError(t('加载我的捐赠渠道失败'));
    } finally {
      setLoading(false);
    }
  };

  const onSubmit = async (values) => {
    setSubmitting(true);
    try {
      const res = await API.post('/api/user/donation-channel', {
        base_url: values.base_url,
        channel_name: values.channel_name,
      });
      const { success, message, data } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      const rewardText =
        Number(data?.reward_quota || 0) > 0
          ? t('，奖励已发放：') + renderQuotaWithPrompt(data.reward_quota)
          : '';
      showSuccess(t('捐赠渠道创建成功') + rewardText);
      formApi?.reset();
      await loadChannels();
    } catch (error) {
      showError(t('提交捐赠渠道失败'));
    } finally {
      setSubmitting(false);
    }
  };

  const onCopyGuideContent = async () => {
    const ok = await copy(donationCopyButtonContent);
    if (ok) {
      showSuccess(t('已复制到剪贴板'));
      return;
    }
    showError(t('复制失败，请手动复制'));
  };

  useEffect(() => {
    loadChannels();
  }, []);

  return (
    <div className='w-full max-w-7xl mx-auto mt-[60px] px-2'>
      <Space vertical align='stretch' spacing='medium'>
        <Card>
          <Space vertical align='stretch'>
            <Typography.Title heading={4} style={{ margin: 0 }}>
              {t('捐赠渠道')}
            </Typography.Title>
            <Typography.Text type='tertiary'>
              {t(
                '提交后系统会将你的渠道加入列表，之后你就可以正常使用 API Key 了。下方默认展示当前全部可用捐赠渠道，你也可以筛选只看自己捐赠的。',
              )}
            </Typography.Text>
            <Typography.Text type='tertiary'>
              {t('如果你捐赠的渠道均不可用，你将无法继续使用。')}
            </Typography.Text>
            <Banner
              type='info'
              bordered
              closeIcon={null}
              title={t(
                '普通用户需要至少保留 1 条本人启用中的捐赠渠道，才能继续通过令牌调用模型。',
              )}
            />
            {(donationGuideText.trim() !== '' || shouldShowCopyButton) && (
              <Card
                size='small'
                bordered
                title={t('捐赠指引')}
                bodyStyle={{ paddingTop: 12, paddingBottom: 12 }}
              >
                <Space vertical align='stretch'>
                  {donationGuideText.trim() !== '' && (
                    <Typography.Paragraph style={{ whiteSpace: 'pre-wrap' }}>
                      {donationGuideText}
                    </Typography.Paragraph>
                  )}
                  {shouldShowCopyButton && (
                    <div>
                      <Button type='primary' theme='solid' onClick={onCopyGuideContent}>
                        {donationCopyButtonText}
                      </Button>
                    </div>
                  )}
                </Space>
              </Card>
            )}
          </Space>
        </Card>

        <Card title={t('提交新的捐赠渠道')}>
          <Form getFormApi={setFormApi} onSubmit={onSubmit}>
            <Form.Input
              field='base_url'
              label={t('Base URL')}
              placeholder={t('请输入上游服务地址，系统会自动提取域名并固定保存为 https://域名/api')}
              rules={[{ required: true, message: t('请输入 Base URL') }]}
            />
            <Form.Input
              field='channel_name'
              label={t('渠道名称')}
              placeholder={t('可选，留空则自动命名')}
            />
            <Button type='primary' htmlType='submit' loading={submitting}>
              {t('提交并校验')}
            </Button>
          </Form>
        </Card>

        <Card
          title={t('当前可用捐赠渠道')}
          headerExtraContent={
            <RadioGroup
              type='button'
              value={scope}
              onChange={(event) => setScope(event.target.value)}
            >
              <Radio value='all'>
                {t('全部')} ({channels.length})
              </Radio>
              <Radio value='mine'>
                {t('仅我捐赠的')} (
                {channels.filter((channel) => channel.is_mine).length})
              </Radio>
            </RadioGroup>
          }
        >
          <Spin spinning={loading}>
            <Table
              dataSource={filteredChannels}
              columns={columns}
              rowKey='id'
              pagination={false}
              empty={
                scope === 'mine'
                  ? t('你还没有可用的捐赠渠道')
                  : t('暂无可用捐赠渠道')
              }
            />
          </Spin>
        </Card>

        <Card title={t('捐赠榜')}>
          <Table
            dataSource={leaderboard}
            columns={leaderboardColumns}
            rowKey='user_id'
            pagination={false}
            empty={t('暂无捐赠记录')}
          />
        </Card>
      </Space>
    </div>
  );
}
