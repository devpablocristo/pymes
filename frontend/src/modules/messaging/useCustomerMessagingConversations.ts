import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  assignCustomerMessagingConversation,
  listCustomerMessagingConversations,
  markCustomerMessagingConversationRead,
  resolveCustomerMessagingConversation,
} from '../../lib/api';
import { queryKeys } from '../../lib/queryKeys';

export function useCustomerMessagingConversations() {
  const queryClient = useQueryClient();

  const conversationsQuery = useQuery({
    queryKey: queryKeys.customerMessaging.conversations,
    queryFn: () => listCustomerMessagingConversations(),
    refetchInterval: 30_000,
  });

  const invalidate = async () => {
    await queryClient.invalidateQueries({ queryKey: queryKeys.customerMessaging.conversations });
  };

  const assignMutation = useMutation({
    mutationFn: ({ id, assignedTo }: { id: string; assignedTo: string }) =>
      assignCustomerMessagingConversation(id, assignedTo),
    onSuccess: invalidate,
  });

  const markReadMutation = useMutation({
    mutationFn: markCustomerMessagingConversationRead,
    onSuccess: invalidate,
  });

  const resolveMutation = useMutation({
    mutationFn: resolveCustomerMessagingConversation,
    onSuccess: invalidate,
  });

  return {
    conversationsQuery,
    assignMutation,
    markReadMutation,
    resolveMutation,
  };
}
