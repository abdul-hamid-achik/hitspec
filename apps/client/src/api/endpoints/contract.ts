import { get, post } from '@/api/client'
import type { ContractResult, ContractStatus, ContractVerifyRequest } from '@/types/api'

export function getContractFiles(): Promise<ContractStatus> {
  return get<ContractStatus>('/api/v1/contract/files')
}

export function verifyContracts(req: ContractVerifyRequest): Promise<ContractResult[]> {
  return post<ContractResult[]>('/api/v1/contract/verify', req)
}
