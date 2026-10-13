import { describe, it, expect } from 'vitest'
import { parseMoney } from './iuranTypes'

describe('parseMoney Regression Tests', () => {
  describe('1. Format desimal API (NUMERIC(15, 2))', () => {
    it('mengubah string desimal "25000.00" menjadi 25000 (bukan 2500000)', () => {
      expect(parseMoney('25000.00')).toBe(25000)
    })

    it('mengubah string desimal "50000.00" menjadi 50000 (bukan 5000000)', () => {
      expect(parseMoney('50000.00')).toBe(50000)
    })

    it('mengubah string desimal "100000.00" menjadi 100000 (bukan 10000000)', () => {
      expect(parseMoney('100000.00')).toBe(100000)
    })

    it('mengubah string desimal "1000000.00" menjadi 1000000 (bukan 100000000)', () => {
      expect(parseMoney('1000000.00')).toBe(1000000)
    })

    it('mengubah string desimal "0.00" menjadi 0', () => {
      expect(parseMoney('0.00')).toBe(0)
    })

    it('mengubah string desimal kecil "50.00" menjadi 50', () => {
      expect(parseMoney('50.00')).toBe(50)
    })
  })

  describe('2. Format Rupiah UI', () => {
    it('mengubah "Rp50.000" menjadi 50000', () => {
      expect(parseMoney('Rp50.000')).toBe(50000)
    })

    it('mengubah "Rp 50.000" (dengan spasi) menjadi 50000', () => {
      expect(parseMoney('Rp 50.000')).toBe(50000)
    })

    it('mengubah "Rp1.500.000" menjadi 1500000', () => {
      expect(parseMoney('Rp1.500.000')).toBe(1500000)
    })

    it('mengubah "1.500.000" (ribuan multi-titik) menjadi 1500000', () => {
      expect(parseMoney('1.500.000')).toBe(1500000)
    })

    it('mengubah "50.000,00" (format Rupiah desimal koma) menjadi 50000', () => {
      expect(parseMoney('50.000,00')).toBe(50000)
    })
  })

  describe('3. Penanganan ambiguitas "50.000" dengan kontrak pemanggil', () => {
    it('"50.000" → 50000 jika kontrak pemanggil menetapkan format Rupiah ("rupiah")', () => {
      expect(parseMoney('50.000', 'rupiah')).toBe(50000)
      expect(parseMoney('50.000', { format: 'rupiah' })).toBe(50000)
    })

    it('"50.000" → 50 jika kontrak pemanggil menetapkan format API / desimal ("api")', () => {
      expect(parseMoney('50.000', 'api')).toBe(50)
      expect(parseMoney('50.000', { format: 'api' })).toBe(50)
    })

    it('"50.000" pada mode default/auto dalam domain iuran diperlakukan sebagai 50000 (Rupiah)', () => {
      expect(parseMoney('50.000')).toBe(50000)
    })
  })

  describe('4. Angka polos tanpa pemisah', () => {
    it('mengubah "50000" menjadi 50000', () => {
      expect(parseMoney('50000')).toBe(50000)
    })

    it('mengubah "25000" menjadi 25000', () => {
      expect(parseMoney('25000')).toBe(25000)
    })

    it('mengubah "1000000" menjadi 1000000', () => {
      expect(parseMoney('1000000')).toBe(1000000)
    })

    it('mengubah "0" menjadi 0', () => {
      expect(parseMoney('0')).toBe(0)
    })
  })

  describe('5. Nilai input number langsung', () => {
    it('mengembalikan nilai integer untuk tipe number 50000', () => {
      expect(parseMoney(50000 as any)).toBe(50000)
    })

    it('mengembalikan 0 untuk number 0', () => {
      expect(parseMoney(0 as any)).toBe(0)
    })
  })

  describe('6. Nilai kosong, invalid, dan unsupported', () => {
    it('string kosong "" menghasilkan 0', () => {
      expect(parseMoney('')).toBe(0)
    })

    it('string spasi "   " menghasilkan 0', () => {
      expect(parseMoney('   ')).toBe(0)
    })

    it('null menghasilkan 0', () => {
      expect(parseMoney(null as any)).toBe(0)
    })

    it('undefined menghasilkan 0', () => {
      expect(parseMoney(undefined as any)).toBe(0)
    })

    it('string non-numerik "invalid" menghasilkan 0', () => {
      expect(parseMoney('invalid')).toBe(0)
    })

    it('string simbol "--" menghasilkan 0', () => {
      expect(parseMoney('--')).toBe(0)
    })
  })

  describe('7. Nilai negatif (ditolak/aman untuk keuangan)', () => {
    it('mengembalikan 0 untuk "-50000"', () => {
      expect(parseMoney('-50000')).toBe(0)
    })

    it('mengembalikan 0 untuk "-50000.00"', () => {
      expect(parseMoney('-50000.00')).toBe(0)
    })

    it('mengembalikan 0 untuk "-Rp50.000"', () => {
      expect(parseMoney('-Rp50.000')).toBe(0)
    })

    it('mengembalikan 0 untuk "Rp -50.000"', () => {
      expect(parseMoney('Rp -50.000')).toBe(0)
    })

    it('mengembalikan 0 untuk number negatif -100', () => {
      expect(parseMoney(-100 as any)).toBe(0)
    })
  })

  describe('8. Pembuktian integritas (tidak ada perkalian 100x atau pembagian 100)', () => {
    it('tidak mengalikan digit sen .00 menjadi ratusan ribu', () => {
      const apiAmount = '50000.00'
      const parsed = parseMoney(apiAmount)
      expect(parsed).not.toBe(5000000)
      expect(parsed).toBe(50000)
    })

    it('tidak membagi angka bulat 50000 menjadi 500', () => {
      expect(parseMoney('50000')).not.toBe(500)
      expect(parseMoney('50000')).toBe(50000)
    })
  })
})
