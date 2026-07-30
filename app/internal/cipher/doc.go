// Package cipher defines the algorithm-neutral encryption boundary used by
// envx. Implementations own key encoding and native ciphertext bytes; the
// package consumer does not own the storage envelope used by secrets.
package cipher
