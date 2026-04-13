package sign

import (
	"strings"
	"testing"
)

func TestEthereumAddressToDIDPKH(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		addr      string
		chainID   int
		want      string
		wantError bool
	}{
		{
			name:    "mainnet_checksummed",
			addr:    "0xb9c5714089478a327f09197987f16f9e5d936e8a",
			chainID: 1,
			want:    "did:pkh:eip155:1:0xB9C5714089478a327F09197987f16f9E5d936E8a",
		},
		{
			name:    "mainnet_lowercase",
			addr:    "0xabcdef0123456789abcdef0123456789abcdef01",
			chainID: 1,
			want:    "did:pkh:eip155:1:0xabCDeF0123456789AbcdEf0123456789aBCDEF01",
		},
		{
			name:    "polygon",
			addr:    "0xb9c5714089478a327f09197987f16f9e5d936e8a",
			chainID: 137,
			want:    "did:pkh:eip155:137:0xB9C5714089478a327F09197987f16f9E5d936E8a",
		},
		{
			name:    "arbitrum",
			addr:    "0xb9c5714089478a327f09197987f16f9e5d936e8a",
			chainID: 42161,
			want:    "did:pkh:eip155:42161:0xB9C5714089478a327F09197987f16f9E5d936E8a",
		},
		{
			name:      "invalid_chainid_zero",
			addr:      "0xb9c5714089478a327f09197987f16f9e5d936e8a",
			chainID:   0,
			wantError: true,
		},
		{
			name:      "invalid_chainid_negative",
			addr:      "0xb9c5714089478a327f09197987f16f9e5d936e8a",
			chainID:   -1,
			wantError: true,
		},
		{
			name:      "invalid_address_format",
			addr:      "not_an_address",
			chainID:   1,
			wantError: true,
		},
		{
			name:    "address_no_0x_accepted",
			addr:    "b9c5714089478a327f09197987f16f9e5d936e8a",
			chainID: 1,
			want:    "did:pkh:eip155:1:0xB9C5714089478a327F09197987f16f9E5d936E8a",
		},
		{
			name:      "invalid_address_too_short",
			addr:      "0xb9c5714089478a327f09197987f16f9e5d936e",
			chainID:   1,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EthereumAddressToDIDPKH(tt.addr, tt.chainID)
			if tt.wantError {
				if err == nil {
					t.Fatalf("EthereumAddressToDIDPKH() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("EthereumAddressToDIDPKH() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("EthereumAddressToDIDPKH() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEthereumAddressFromDIDPKH(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		kid           string
		wantAddr      string
		wantChainID   int
		wantError     bool
		errorContains string
	}{
		{
			name:        "mainnet_checksummed",
			kid:         "did:pkh:eip155:1:0xB9C5714089478a327F09197987f16f9E5d936E8a",
			wantAddr:    "0xB9C5714089478a327F09197987f16f9E5d936E8a",
			wantChainID: 1,
		},
		{
			name:        "mainnet_lowercase_accepted",
			kid:         "did:pkh:eip155:1:0xb9c5714089478a327f09197987f16f9e5d936e8a",
			wantAddr:    "0xB9C5714089478a327F09197987f16f9E5d936E8a",
			wantChainID: 1,
		},
		{
			name:        "polygon",
			kid:         "did:pkh:eip155:137:0xB9C5714089478a327F09197987f16f9E5d936E8a",
			wantAddr:    "0xB9C5714089478a327F09197987f16f9E5d936E8a",
			wantChainID: 137,
		},
		{
			name:        "arbitrum",
			kid:         "did:pkh:eip155:42161:0xabCDeF0123456789AbcdEf0123456789aBCDEF01",
			wantAddr:    "0xabCDeF0123456789AbcdEf0123456789aBCDEF01",
			wantChainID: 42161,
		},
		{
			name:          "not_did_pkh",
			kid:           "did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK",
			wantError:     true,
			errorContains: "did:pkh",
		},
		{
			name:          "wrong_namespace",
			kid:           "did:pkh:bip122:000000000019d6689c085ae165831e93:128Lkh3S7CkDTBZ8W7BbpsN3YYizJMp8p6",
			wantError:     true,
			errorContains: "eip155",
		},
		{
			name:          "invalid_format_missing_parts",
			kid:           "did:pkh:eip155:1",
			wantError:     true,
			errorContains: "3 colon-separated parts",
		},
		{
			name:          "invalid_chainid_not_number",
			kid:           "did:pkh:eip155:mainnet:0xB9C5714089478a327F09197987f16f9E5d936E8a",
			wantError:     true,
			errorContains: "chainID",
		},
		{
			name:          "invalid_chainid_zero",
			kid:           "did:pkh:eip155:0:0xB9C5714089478a327F09197987f16f9E5d936E8a",
			wantError:     true,
			errorContains: "chainID",
		},
		{
			name:          "invalid_address",
			kid:           "did:pkh:eip155:1:not_an_address",
			wantError:     true,
			errorContains: "address",
		},
		{
			name:          "wrong_checksum_rejected",
			kid:           "did:pkh:eip155:1:0xb9C5714089478A327f09197987f16F9e5d936e8A",
			wantError:     true,
			errorContains: "checksum",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAddr, gotChainID, err := EthereumAddressFromDIDPKH(tt.kid)
			if tt.wantError {
				if err == nil {
					t.Fatalf("EthereumAddressFromDIDPKH() expected error, got nil")
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("error = %q, want substring %q", err.Error(), tt.errorContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("EthereumAddressFromDIDPKH() error = %v", err)
			}
			if gotAddr != tt.wantAddr {
				t.Errorf("address = %q, want %q", gotAddr, tt.wantAddr)
			}
			if gotChainID != tt.wantChainID {
				t.Errorf("chainID = %d, want %d", gotChainID, tt.wantChainID)
			}
		})
	}
}

func TestEthereumDIDPKHRoundTrip(t *testing.T) {
	t.Parallel()

	addresses := []string{
		"0xb9c5714089478a327f09197987f16f9e5d936e8a",
		"0xAbCdef0123456789aBcdef0123456789AbCDEf01",
		"0x0000000000000000000000000000000000000000",
		"0xFFfFfFffFFfffFFfFFfFFFFFffFFFffffFfFFFfF",
	}

	chainIDs := []int{1, 5, 137, 42161, 11155111}

	for _, addr := range addresses {
		for _, chainID := range chainIDs {
			kid, err := EthereumAddressToDIDPKH(addr, chainID)
			if err != nil {
				t.Fatalf("EthereumAddressToDIDPKH(%q, %d) error = %v", addr, chainID, err)
			}

			gotAddr, gotChainID, err := EthereumAddressFromDIDPKH(kid)
			if err != nil {
				t.Fatalf("EthereumAddressFromDIDPKH(%q) error = %v", kid, err)
			}

			// Addresses are normalized to checksum, so compare case-insensitively
			if !strings.EqualFold(gotAddr, addr) {
				t.Errorf("round-trip address mismatch: got %q, want %q", gotAddr, addr)
			}

			if gotChainID != chainID {
				t.Errorf("round-trip chainID mismatch: got %d, want %d", gotChainID, chainID)
			}
		}
	}
}
