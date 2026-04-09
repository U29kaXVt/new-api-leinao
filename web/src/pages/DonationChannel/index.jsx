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

import React, { useEffect, useMemo, useState } from 'react';
import {
  Banner,
  Button,
  Card,
  Form,
  Space,
  Spin,
  Table,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import {
  API,
  showError,
  showSuccess,
  timestamp2string,
  renderQuotaWithPrompt,
} from '../../helpers';
import { useTranslation } from 'react-i18next';

export default function DonationChannel() {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [channels, setChannels] = useState([]);
  const [formApi, setFormApi] = useState(null);

  const columns = useMemo(
    () => [
      {
        title: t('名称'),
        dataIndex: 'name',
      },
      {
        title: t('Base URL'),
        dataIndex: 'base_url',
        render: (value) => (
          <Typography.Text copyable={{ content: value }}>
            {value}
          </Typography.Text>
        ),
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
        title: t('模型列表'),
        dataIndex: 'models',
        render: (models) =>
          Array.isArray(models) && models.length > 0
            ? models.join(', ')
            : t('未返回'),
      },
      {
        title: t('创建时间'),
        dataIndex: 'created_time',
        render: (value) => timestamp2string(value),
      },
    ],
    [t],
  );

  const loadChannels = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/user/donation-channel');
      const { success, message, data } = res.data;
      if (!success) {
        showError(message);
        return;
      }
      setChannels(data || []);
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
        key: values.key,
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

  useEffect(() => {
    loadChannels();
  }, []);

  return (
    <div className='w-full max-w-7xl mx-auto mt-[60px] px-2'>
      <Space vertical align='stretch' spacing='medium'>
        <Card>
          <Space vertical align='stretch'>
            <Typography.Title heading={4} style={{ margin: 0 }}>
              {t('我的捐赠渠道')}
            </Typography.Title>
            <Typography.Text type='tertiary'>
              {t(
                '提交后系统会基于管理员配置的模板渠道克隆你的专属渠道，并校验上游模型列表。密钥不会在列表中返回。',
              )}
            </Typography.Text>
            <Banner
              type='info'
              bordered
              closeIcon={null}
              title={t(
                '普通用户需要至少保留 1 条本人启用中的捐赠渠道，才能继续通过令牌调用模型。',
              )}
            />
          </Space>
        </Card>

        <Card title={t('提交新的捐赠渠道')}>
          <Form getFormApi={setFormApi} onSubmit={onSubmit}>
            <Form.Input
              field='base_url'
              label={t('Base URL')}
              placeholder={t('请输入上游服务的 Base URL')}
              rules={[{ required: true, message: t('请输入 Base URL') }]}
            />
            <Form.Input
              field='key'
              mode='password'
              label={t('密钥')}
              placeholder={t('请输入上游服务密钥')}
              rules={[{ required: true, message: t('请输入密钥') }]}
            />
            <Button type='primary' htmlType='submit' loading={submitting}>
              {t('提交并校验')}
            </Button>
          </Form>
        </Card>

        <Card title={t('当前有效捐赠渠道')}>
          <Spin spinning={loading}>
            <Table
              dataSource={channels}
              columns={columns}
              rowKey='id'
              pagination={false}
              empty={t('暂无有效捐赠渠道')}
            />
          </Spin>
        </Card>
      </Space>
    </div>
  );
}
