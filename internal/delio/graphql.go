package delio

const productPartsFragment = `fragment ProductParts on Product {
  sku
  id
  key
  name
  metaTitle
  metaDescription
  description
  slug
  productType
  depositFee {
    value {
      centAmount
      currencyCode
      ... on Money {
        centAmount
        currencyCode
        fractionDigits
        type
        __typename
      }
      ... on HighPrecisionMoney {
        centAmount
        currencyCode
        fractionDigits
        preciseAmount
        type
        __typename
      }
      __typename
    }
    __typename
  }
  bundleItems {
    product {
      depositFee {
        value {
          centAmount
          currencyCode
          ... on Money {
            centAmount
            currencyCode
            fractionDigits
            type
            __typename
          }
          ... on HighPrecisionMoney {
            centAmount
            currencyCode
            fractionDigits
            preciseAmount
            type
            __typename
          }
          __typename
        }
        __typename
      }
      attributes {
        bi_alcohol_percent
        bi_allergens_contain
        bi_allergens_may_contain
        bi_carbohydrate_content
        bi_carbohydrate_content_sugars
        bi_country_of_origin
        bi_fat_content_saturated_fatty_acids
        bi_fat_content_total
        bi_ingredients
        bi_protein_content
        bi_salt_content
        bi_storage_method
        bi_supplier_name
        contain_unit
        contains_alcohol
        discountable
        energy_drink
        energy_value_kcal
        energy_value_kj
        long_description
        net_contain
        short_shelf_life
        tags {
          key
          label
          __typename
        }
        __typename
      }
      categoriesIds
      description
      id
      imagesUrls
      isPromotedProduct
      isPublished
      key
      name
      sku
      __typename
    }
    quantity
    sku
    __typename
  }
  price {
    multibuy {
      value {
        centAmount
        currencyCode
        fractionDigits
        type
        __typename
      }
      description
      triggerQuantity
      maxQuantity
      __typename
    }
    discounted {
      discount {
        id
        validFrom
        validUntil
        isActive
        name
        description
        value {
          type
          ... on AbsoluteDiscountValue {
            money {
              centAmount
              currencyCode
              fractionDigits
              type
              __typename
            }
            type
            __typename
          }
          ... on RelativeDiscountValue {
            permyriad
            __typename
          }
          __typename
        }
        __typename
      }
      value {
        centAmount
        currencyCode
        fractionDigits
        type
        __typename
      }
      __typename
    }
    value {
      centAmount
      currencyCode
      fractionDigits
      type
      __typename
    }
    __typename
  }
  availableQuantity
  attributes {
    bi_alcohol_percent
    bi_allergens_contain
    bi_allergens_may_contain
    bi_carbohydrate_content
    bi_carbohydrate_content_sugars
    bi_country_of_origin
    bi_fat_content_saturated_fatty_acids
    bi_fat_content_total
    bi_ingredients
    bi_protein_content
    bi_salt_content
    bi_storage_method
    bi_supplier_name
    contain_unit
    contains_alcohol
    discountable
    energy_drink
    energy_value_kcal
    energy_value_kj
    long_description
    net_contain
    short_shelf_life
    tags {
      key
      label
      __typename
    }
    weight_surcharge_percent
    __typename
  }
  imagesUrls
  categoriesIds
  isPublished
  isFavourite
  isPromotedProduct
  __typename
}`

const cartFragment = `fragment CartFragment on CustomerCart {
  totalDepositFee {
    value {
      centAmount
      currencyCode
      fractionDigits
      type
      __typename
    }
    __typename
  }
  tipForRider {
    value {
      centAmount
      currencyCode
      __typename
    }
    __typename
  }
  packagingFee {
    discounted {
      value {
        centAmount
        currencyCode
        fractionDigits
        type
        __typename
      }
      __typename
    }
    value {
      centAmount
      currencyCode
      fractionDigits
      type
      __typename
    }
    __typename
  }
  shippingAddress {
    pickupPointId
    streetName
    streetNumber
    postalCode
    city
    countryCode
    apartment
    phoneNumber {
      nationalNumber
      countryCode
      __typename
    }
    floor
    firstName
    lastName
    lat
    long
    deliveryNotes
    locationType
    locationName
    __typename
  }
  billingAddress {
    ... on CompanyBillingAddress {
      streetName
      streetNumber
      postalCode
      city
      countryCode
      apartment
      vatId
      email
      company
      __typename
    }
    ... on PersonalBillingAddress {
      streetName
      streetNumber
      postalCode
      city
      countryCode
      apartment
      email
      firstName
      lastName
      __typename
    }
    __typename
  }
  paymentInfo {
    payments {
      id
      __typename
    }
    __typename
  }
  darkstoreKey
  context
  deliveryScheduleSlot {
    dateFrom
    dateTo
    __typename
  }
  shippingMethod {
    name
    key
    type
    price {
      freeAbove {
        type
        currencyCode
        centAmount
        fractionDigits
        __typename
      }
      value {
        type
        currencyCode
        centAmount
        fractionDigits
        __typename
      }
      __typename
    }
    __typename
  }
  discountCodes {
    state
    code
    description
    discountValue {
      ... on AbsoluteDiscountValue {
        type
        money {
          currencyCode
          centAmount
          fractionDigits
          __typename
        }
        __typename
      }
      ... on RelativeDiscountValue {
        type
        permyriad
        __typename
      }
      __typename
    }
    __typename
  }
  id
  version
  totalPrice {
    currencyCode
    centAmount
    __typename
  }
  lineItems {
    id
    quantity
    product {
      ...ProductParts
      __typename
    }
    totalPrice {
      centAmount
      currencyCode
      __typename
    }
    __typename
  }
  rewards {
    ... on DiscountReward {
      afterReachThresholdText
      beforeReachThresholdText
      discountAdditionalInfo
      discountDescription
      discountMinimumOrderValue {
        centAmount
        currencyCode
        fractionDigits
        type
        __typename
      }
      discountName
      discountValue {
        ... on AbsoluteRewardValue {
          money {
            centAmount
            currencyCode
            fractionDigits
            type
            __typename
          }
          type
          __typename
        }
        ... on RelativeRewardValue {
          permyriad
          type
          __typename
        }
        __typename
      }
      minimumCartValue {
        centAmount
        currencyCode
        fractionDigits
        type
        __typename
      }
      rewardIconUrl
      type
      __typename
    }
    ... on ShippingDiscountReward {
      discount {
        centAmount
        currencyCode
        fractionDigits
        type
        __typename
      }
      minimumCartValue {
        centAmount
        currencyCode
        fractionDigits
        type
        __typename
      }
      __typename
    }
    __typename
  }
  weightSurcharge {
    value {
      centAmount
      currencyCode
      __typename
    }
    __typename
  }
  __typename
}`

const productSearchQuery = `query ProductSearch($query: String!, $limit: Int!, $offset: Int!, $coordinates: CoordinatesInput) {
  productSearch(
    query: $query
    limit: $limit
    offset: $offset
    coordinates: $coordinates
  ) {
    attributionToken
    total
    results {
      ...ProductParts
      __typename
    }
    __typename
  }
}

` + productPartsFragment

const productQuery = `query Product($slug: String, $sku: String, $coordinates: CoordinatesInput) {
  product(slug: $slug, sku: $sku, coordinates: $coordinates) {
    ...ProductParts
    __typename
  }
}

` + productPartsFragment

const currentCartQuery = `query CurrentCart {
  currentCart {
    ...CartFragment
    __typename
  }
}

` + cartFragment + `

` + productPartsFragment

const updateCurrentCartMutation = `mutation UpdateCurrentCart($cartId: ID!, $actions: [CustomerCartUpdateAction!]!) {
  updateCart(cartId: $cartId, actions: $actions) {
    ...CartFragment
    __typename
  }
}

` + cartFragment + `

` + productPartsFragment

const deliveryScheduleSlotsQuery = `query DeliveryScheduleSlots($coordinates: CoordinatesInput) {
  deliveryScheduleSlots(coordinates: $coordinates) {
    dateFrom
    dateTo
    available
    bookableUntil
    __typename
  }
}`

const paymentSettingsQuery = `query PaymentSettings {
  paymentSettings {
    adyenClientKey
    __typename
  }
}`

const createPaymentMutation = `mutation CreatePayment($cartId: ID!) {
  createPayment(cartId: $cartId) {
    paymentId
    __typename
  }
}`

const paymentMethodsQuery = `query PaymentMethods($cartId: String!, $paymentId: String!) {
  getPaymentMethods(cartId: $cartId, paymentId: $paymentId) {
    adyenResponse
    __typename
  }
}`

const makePaymentMutation = `mutation MakePayment($paymentId: String!, $cartId: String!, $paymentConfig: MakeAdyenPaymentArgs!) {
  makePayment(paymentId: $paymentId, cartId: $cartId, paymentConfig: $paymentConfig) {
    adyenResponse
    __typename
  }
}`

const customerDefaultBillingAddressQuery = `query CustomerDefaultBillingAddress {
  customerDefaultBillingAddress {
    billingAddress {
      ... on CompanyBillingAddress {
        streetName
        streetNumber
        postalCode
        city
        countryCode
        apartment
        vatId
        email
        company
        __typename
      }
      ... on PersonalBillingAddress {
        streetName
        streetNumber
        postalCode
        city
        countryCode
        apartment
        email
        firstName
        lastName
        __typename
      }
      __typename
    }
    id
    __typename
  }
}`

const customerShippingAddressQuery = `query CustomerShippingAddress {
  defaultShippingAddress {
    apartment
    city
    countryCode
    deliveryNotes
    entrance
    firstName
    floor
    id
    isDefault
    lastName
    lat
    locationName
    locationType
    long
    phoneNumber {
      countryCode
      nationalNumber
      __typename
    }
    pickupPointId
    postalCode
    streetName
    streetNumber
    __typename
  }
}`
