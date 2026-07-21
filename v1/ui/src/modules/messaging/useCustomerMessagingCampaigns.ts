import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { createCustomerMessagingCampaign, listCustomerMessagingCampaigns, sendCustomerMessagingCampaign } from '../../lib/api';
import { queryKeys } from '../../lib/queryKeys';

export type MessagingCampaignDraft = {
  name: string;
  template_name: string;
  template_language: string;
  template_params: string;
  tag_filter: string;
};

export const initialMessagingCampaignDraft: MessagingCampaignDraft = {
  name: '',
  template_name: '',
  template_language: 'es',
  template_params: '',
  tag_filter: '',
};

export function useCustomerMessagingCampaigns(draft: MessagingCampaignDraft, onCreated: () => void) {
  const queryClient = useQueryClient();

  const campaignsQuery = useQuery({
    queryKey: queryKeys.customerMessaging.campaigns,
    queryFn: listCustomerMessagingCampaigns,
    refetchInterval: 30_000,
  });

  const invalidate = async () => {
    await queryClient.invalidateQueries({ queryKey: queryKeys.customerMessaging.campaigns });
  };

  const createMutation = useMutation({
    mutationFn: () =>
      createCustomerMessagingCampaign({
        name: draft.name.trim(),
        template_name: draft.template_name.trim(),
        template_language: draft.template_language.trim() || 'es',
        template_params: draft.template_params
          .split(',')
          .map((value) => value.trim())
          .filter(Boolean),
        tag_filter: draft.tag_filter.trim() || undefined,
      }),
    onSuccess: async () => {
      onCreated();
      await invalidate();
    },
  });

  const sendMutation = useMutation({
    mutationFn: sendCustomerMessagingCampaign,
    onSuccess: invalidate,
  });

  return {
    campaignsQuery,
    createMutation,
    sendMutation,
  };
}
