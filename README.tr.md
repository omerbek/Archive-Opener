# Arşiv Açıcı

*[English](README.md) | Türkçe*

Windows için WinRAR benzeri, kurulum gerektirmeyen bir arşiv açma programı. Tek bir `.exe` dosyasıdır.

## Desteklenen biçimler

- **RAR** ve **RAR5**: çok parçalı arşivler (`.part1.rar`, `.r00`) ve kendi kendini açan `.exe` arşivleri dahil
- **ZIP**: eski Türkçe Windows dosya adları (CP857) ve şifreli ZIP (ZipCrypto ve AES)
- **7z**: şifreli içerik ve şifreli dosya listesi dahil
- **TAR**, **GZ**, **BZ2**, **XZ**, **ZST** (`.tar.gz`, `.tgz` gibi birleşik biçimler de)

Program arşivleri yalnızca açar, arşiv oluşturmaz. Çok eski RAR 1.5 arşivleri desteklenmez.

## Özellikler

- Arşivin içinde klasör klasör gezinme, sütunlara göre sıralama, Explorer simgeleri
- Dosyaya çift tıklayınca kendi programıyla açılır; arşivin içindeki arşiv programın içinde açılır
- **Çıkar…**: hedef klasör ve aynı adlı dosyalar için seçenekler (sor / üzerine yaz / atla)
- **Klasöre çıkar**: tek tıkla, arşivin yanındaki aynı adlı klasöre çıkarır
- **Test et**: arşivdeki bozuk dosyaları bulur
- İlerleme penceresi, hız göstergesi ve iptal
- Sürükle-bırak ile açma
- İsteğe bağlı Windows tanıtımı: "Birlikte aç" listesi ve sağ tık menüsünde "Buraya çıkar" / "Klasöre çıkar". Yalnızca kullanıcı hesabına yazılır, menüden kaldırılabilir.
- Güvenlik: hedef klasörün dışına yazmaya çalışan yollar (`../`) ve sembolik bağlantılar çıkarılmaz

## İndirme

Derlenmiş `ArsivAcici.exe` dosyasını [Releases](../../releases) sayfasından indirebilirsiniz. Program imzasız olduğu için Windows SmartScreen ilk açılışta uyarı verebilir; "Ek bilgi → Yine de çalıştır" ile açılır.

Komut satırından kullanım:

```
ArsivAcici.exe dosya.rar        arşivi pencerede açar
ArsivAcici.exe -x dosya.rar     arşivin yanındaki "dosya" klasörüne çıkarır
ArsivAcici.exe -xh dosya.rar    arşivin bulunduğu klasöre çıkarır
```

## Derleme

Go 1.21 veya üstü gerekir. Linux, macOS veya Windows'ta derlenebilir:

```
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-H windowsgui -s -w" -o ArsivAcici.exe ./app
```

Simge ve manifest `app/rsrc_windows_amd64.syso` dosyasında gömülüdür. Değiştirmek için:

```
go run ./cmd/mkicon app/app.ico
go install github.com/akavel/rsrc@latest
rsrc -arch amd64 -manifest app/app.manifest -ico app/app.ico -o app/rsrc_windows_amd64.syso
```

`go.mod` dosyasındaki `replace` satırları, bazı bağımlılıkları doğrudan GitHub kaynaklarından almak ve yalnızca testlerde kullanılan modülleri boş yer tutucularla (`stubs/`) karşılamak içindir.

## Proje yapısı

| Klasör | İçerik |
|---|---|
| `arc/` | Arşiv okuma ve güvenli çıkarma (platformdan bağımsız) |
| `app/` | Windows arayüzü ([lxn/walk](https://github.com/lxn/walk)) |
| `cmd/arctest/` | Arşivleri komut satırından test etmek için küçük araç |
| `cmd/mkicon/` | Uygulama simgesini çizen araç |

## Kullanılan kütüphaneler

- [nwaples/rardecode](https://github.com/nwaples/rardecode): RAR ve RAR5
- [bodgit/sevenzip](https://github.com/bodgit/sevenzip): 7z
- [ulikunitz/xz](https://github.com/ulikunitz/xz), [klauspost/compress](https://github.com/klauspost/compress): XZ, LZMA, Zstandard
- [lxn/walk](https://github.com/lxn/walk): Windows arayüzü

## Lisans

MIT. Ayrıntılar için [LICENSE](LICENSE) dosyasına bakın.
