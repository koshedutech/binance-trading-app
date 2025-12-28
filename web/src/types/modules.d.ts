// Module declarations for packages without type definitions
// These suppress TypeScript errors for packages that don't include their own types

declare module '@xyflow/react' {
  export const ReactFlow: any;
  export const Background: any;
  export const Controls: any;
  export const MiniMap: any;
  export const Panel: any;
  export const Handle: any;
  export const Position: any;
  export const BackgroundVariant: any;
  export const useNodesState: any;
  export const useEdgesState: any;
  export const addEdge: any;
  export const ReactFlowProvider: any;
  export type Node<T = any> = any;
  export type Edge<T = any> = any;
  export type Connection = any;
  export type NodeProps<T = any> = any;
}

declare module 'axios' {
  export interface AxiosRequestConfig {
    url?: string;
    method?: string;
    baseURL?: string;
    headers?: Record<string, string>;
    params?: any;
    data?: any;
    timeout?: number;
    withCredentials?: boolean;
    _retry?: boolean;
  }

  export interface AxiosResponse<T = any> {
    data: T;
    status: number;
    statusText: string;
    headers: Record<string, string>;
    config: AxiosRequestConfig;
  }

  export interface AxiosError<T = any> extends Error {
    config?: AxiosRequestConfig;
    code?: string;
    request?: any;
    response?: AxiosResponse<T>;
    isAxiosError: boolean;
  }

  export interface AxiosInstance {
    defaults: AxiosRequestConfig;
    interceptors: {
      request: {
        use: (onFulfilled?: (config: AxiosRequestConfig) => AxiosRequestConfig | Promise<AxiosRequestConfig>, onRejected?: (error: any) => any) => number;
      };
      response: {
        use: (onFulfilled?: (response: AxiosResponse) => AxiosResponse | Promise<AxiosResponse>, onRejected?: (error: any) => any) => number;
      };
    };
    get<T = any>(url: string, config?: AxiosRequestConfig): Promise<AxiosResponse<T>>;
    post<T = any>(url: string, data?: any, config?: AxiosRequestConfig): Promise<AxiosResponse<T>>;
    put<T = any>(url: string, data?: any, config?: AxiosRequestConfig): Promise<AxiosResponse<T>>;
    delete<T = any>(url: string, config?: AxiosRequestConfig): Promise<AxiosResponse<T>>;
    patch<T = any>(url: string, data?: any, config?: AxiosRequestConfig): Promise<AxiosResponse<T>>;
    (config: AxiosRequestConfig): Promise<AxiosResponse>;
  }

  function create(config?: AxiosRequestConfig): AxiosInstance;
  function isAxiosError(payload: any): payload is AxiosError;

  const axios: AxiosInstance & {
    create: typeof create;
    isAxiosError: typeof isAxiosError;
  };

  export default axios;
  export { create, isAxiosError };
}
