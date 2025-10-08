import { ApolloClient, InMemoryCache, HttpLink } from '@apollo/client'

const GRAPHQL_URL = import.meta.env.VITE_GRAPHQL_URL || 'http://localhost:8080/query'

const httpLink = new HttpLink({
  uri: GRAPHQL_URL,
})

export const client = new ApolloClient({
  link: httpLink,
  cache: new InMemoryCache(),
  defaultOptions: {
    watchQuery: {
      fetchPolicy: 'network-only',
    },
    query: {
      fetchPolicy: 'network-only',
    },
  },
})

